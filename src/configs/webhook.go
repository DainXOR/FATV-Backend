package configs

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"dainxor/atv/logger"
	"dainxor/atv/utils"

	amqp "github.com/rabbitmq/amqp091-go"
)

//
// ───────────────────────────────────────────────────
//   TYPES
// ───────────────────────────────────────────────────
//

type payload struct {
	Data   any       `json:"data"`
	SentAt time.Time `json:"sent_at"`
}

type connectionConfig struct {
	URL      string `json:"url"`
	Exchange string `json:"exchange"`

	// Backoff logic configs
	BackoffBaseTime int     `json:"backoff_base_time "` // base delay retry_seconds
	BackoffMaxTime  int     `json:"max_retry_seconds"`  // cap
	BackoffFactor   float64 `json:"backoff_factor"`     // usually 2
	JitterEnabled   bool    `json:"jitter_enabled"`

	// Queue publich configs
	PublishMaxAttemps int `json:"publish_max_attemps"` // publish retry count
}

type connectionState struct {
	cfg        connectionConfig
	conn       *amqp.Connection
	ch         *amqp.Channel
	open       bool
	mutex      sync.RWMutex
	shutdownCh chan struct{}
}

type webhookNS struct {
	state *connectionState
	mutex sync.RWMutex
}

var WebHooks webhookNS

//
// ───────────────────────────────────────────────────
//   INIT CONFIG
// ───────────────────────────────────────────────────
//

func init() {
	cfg, err := loadWebhookConfig()
	if err != nil {
		logger.Error("Failed to load WH config:", err)
	}

	WebHooks.startBroker(cfg)
}

func loadDefaultConfig() connectionConfig {
	return connectionConfig{
		URL:               "",
		Exchange:          "default-ex",
		BackoffBaseTime:   5,
		BackoffMaxTime:    30,
		BackoffFactor:     2,
		JitterEnabled:     true,
		PublishMaxAttemps: 3,
	}
}
func loadEnvConfig() (connectionConfig, error) {
	cfg := loadDefaultConfig()

	envUrl, ok := os.LookupEnv("WH_URL")
	if !ok {
		return cfg, errors.New("missing WH_URL")
	}
	cfg.URL = envUrl

	exchange, ok := os.LookupEnv("WH_EXCHANGE")
	if !ok {
		return cfg, errors.New("missing WH_EXCHANGE")
	}
	cfg.Exchange = exchange

	// Optional values with defaults
	cfg.BackoffBaseTime = utils.GetEnvInt("WH_RETRY_SECONDS", cfg.BackoffBaseTime)
	cfg.BackoffMaxTime = utils.GetEnvInt("WH_MAX_RETRY_SECONDS", cfg.BackoffMaxTime)
	cfg.BackoffFactor = utils.GetEnvFloat("WH_BACKOFF_FACTOR", cfg.BackoffFactor)
	cfg.JitterEnabled = utils.GetEnvBool("WH_JITTER_ENABLED", cfg.JitterEnabled)
	cfg.PublishMaxAttemps = utils.GetEnvInt("WH_MAX_PUBLISH_RETRY", cfg.PublishMaxAttemps)

	return cfg, nil
}

func loadWebhookConfig() (*connectionState, error) {
	cfg, err := loadEnvConfig()
	if err != nil {
		logger.Warning("Using default webhook config:", err)
		def := loadDefaultConfig()

		return &connectionState{
			cfg:        def,
			shutdownCh: make(chan struct{}),
		}, err
	}

	return &connectionState{
		cfg:        cfg,
		shutdownCh: make(chan struct{}),
	}, nil
}

//
// ───────────────────────────────────────────────────
//   START BROKERS (AUTO-RECONNECT)
// ───────────────────────────────────────────────────
//

func (wh *webhookNS) startBroker(st *connectionState) {
	wh.state = st
	go wh.runBroker(st)
}

func (wh *webhookNS) runBroker(st *connectionState) {
	base := time.Duration(st.cfg.BackoffBaseTime) * time.Second
	if base <= 0 {
		base = 3 * time.Second
	}

	attempt := 0

	for {
		select {
		case <-st.shutdownCh:
			logger.Info("webhook: shutting down broker")
			return

		default:
			// Attempt connection/setup
			err := wh.connectAndSetup(st)
			if err != nil {
				attempt++

				// Exponential backoff
				backoff := min(base*(1<<(attempt-1)), time.Minute)

				// Jitter ±20%
				const jitterPercentage = 20

				minimumDelay := int64(backoff) * (100 - jitterPercentage) / 100
				maximumDelay := int64(backoff) * (100 + jitterPercentage) / 100

				delayRange := maximumDelay - minimumDelay
				if delayRange <= 0 {
					delayRange = 1
				}

				delay := time.Duration(rand.Int63n(delayRange) + minimumDelay)

				//logger.Warningf(
				//	"webhook: connection/setup failed: %v — retrying in %v (attempt %d)",
				//	err, delay, attempt,
				//)

				time.Sleep(delay)
				continue
			}

			// Success — reset backoff
			attempt = 0
			logger.Info("webhook: broker connected & ready")

			// Wait until connection dies
			err = <-st.conn.NotifyClose(make(chan *amqp.Error))
			logger.Warningf("webhook: connection lost: %v", err)

			st.mutex.Lock()
			st.open = false
			st.mutex.Unlock()

			// Loop → reconnect
		}
	}
}

//
// ───────────────────────────────────────────────────
//   CONNECT + SETUP EXCHANGE/QUEUE/BINDINGS
// ───────────────────────────────────────────────────
//

func (wh *webhookNS) connectAndSetup(st *connectionState) error {
	cfg := st.cfg

	// 1. Connect
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return fmt.Errorf("webhook dial: %w", err)
	}

	// 2. Open channel
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("webhook channel: %w", err)
	}

	// 3. Declare Topic Exchange (producer responsibility)
	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		"topic",
		true,  // durable
		false, // auto-delete
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("webhook exchange: %w", err)
	}

	// 4. Store connection
	st.mutex.Lock()
	st.conn = conn
	st.ch = ch
	st.open = true
	st.mutex.Unlock()

	return nil
}

//
// ───────────────────────────────────────────────────
//   VERIFY CONNECTION
// ───────────────────────────────────────────────────
//

func (wh *webhookNS) IsReady() bool {
	st := wh.state
	if st == nil {
		return false
	}
	st.mutex.RLock()
	defer st.mutex.RUnlock()
	return st.open
}

// ───────────────────────────────────────────────────
//
//	SEND MESSAGE (SAFE / AUTO-RETRY)
//
// ───────────────────────────────────────────────────
func (wh *webhookNS) SendTo(routing string, data any) error {
	return wh.SendTo(routing, data)
}

func (wh *webhookNS) internalSend(routing string, data any) error {
	state := wh.state
	if state == nil {
		logger.Error("webhook: state not initialized")
		return errors.New("webhook: state not initialized")
	}

	// Encode payload

	body, err := json.Marshal(payload{Data: data, SentAt: time.Now()})
	if err != nil {
		logger.Errorf("webhook marshal: %v", err)
		return fmt.Errorf("webhook marshal: %w", err)
	}

	retries := state.cfg.PublishMaxAttemps
	if retries < 1 {
		retries = 1
	}

	for attempt := 1; attempt <= retries; attempt++ {

		state.mutex.RLock()
		open := state.open
		channel := state.ch
		state.mutex.RUnlock()

		if !open || channel == nil {
			// No channel → wait for reconnect
			time.Sleep(200 * time.Millisecond)
			continue
		}

		pubErr := channel.Publish(
			state.cfg.Exchange,
			routing,
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			},
		)

		if pubErr == nil {
			return nil // success
		}

		logger.Warningf(
			"webhook: publish attempt %d/%d failed: %v",
			attempt, retries, pubErr,
		)

		time.Sleep(time.Duration(attempt*150) * time.Millisecond)
	}

	logger.Error("webhook: max publish retries reached")
	return errors.New("webhook: max publish retries reached")
}

//
// ───────────────────────────────────────────────────
//   GRACEFUL CLOSE
// ───────────────────────────────────────────────────
//

func (wh *webhookNS) Close() {
	state := wh.state
	if state == nil {
		return
	}

	close(state.shutdownCh)

	state.mutex.Lock()
	defer state.mutex.Unlock()

	if state.ch != nil {
		state.ch.Close()
		state.ch = nil
	}

	if state.conn != nil {
		state.conn.Close()
		state.conn = nil
	}

	state.open = false

	logger.Info("webhook: closed")
}
