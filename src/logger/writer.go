package logger

import (
	"fmt"
	"os"
	"path/filepath"
)

type CloseFunc func() error

type Writer interface {
	CreationError() error
	Write(text string) error
	Close() error
}
type WriterBuilder interface {
	New() Writer
}

type consoleWriter struct {
	formatString string
	err          error
}

func (w *consoleWriter) CreationError() error {
	return w.err
}
func (w *consoleWriter) Write(text string) error {
	if w.formatString == "" {
		return fmt.Errorf("format string is empty")
	}

	_, err := fmt.Printf(w.formatString, text)
	return err
}
func (w *consoleWriter) Close() error {
	// No resources to close for console writer
	return nil
}

type ConsoleWriterBuilder struct {
	writer consoleWriter
}

func (b ConsoleWriterBuilder) NewLine() ConsoleWriterBuilder {
	if b.writer.formatString == "" {
		b.writer.formatString = "%s\n"
	} else {
		b.writer.formatString += "\n"
	}
	return b
}
func (b ConsoleWriterBuilder) FormatString(format string) ConsoleWriterBuilder {
	b.writer.formatString = format
	return b
}
func (b ConsoleWriterBuilder) New() Writer {
	if b.writer.formatString == "" {
		b.writer.formatString = "%s"
	}

	return &consoleWriter{
		formatString: b.writer.formatString,
		err:          nil,
	}
}

type fileWriter struct {
	formatString string
	FilePath     string
	file         *os.File
	err          error
}

func (w *fileWriter) CreationError() error {
	return w.err
}
func (w *fileWriter) Write(text string) error {
	if w.file == nil {
		if w.err != nil {
			return w.err
		}
		return fmt.Errorf("log file is not open")
	}
	_, err := fmt.Fprintf(w.file, w.formatString, text)
	return err
}
func (w *fileWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

type FileWriterBuilder struct {
	writer fileWriter
}

func (b FileWriterBuilder) FilePath(path string) FileWriterBuilder {
	b.writer.FilePath = path
	return b
}
func (b FileWriterBuilder) NewLine() FileWriterBuilder {
	if b.writer.formatString == "" {
		b.writer.formatString = "%s\n"
	} else {
		b.writer.formatString += "\n"
	}

	return b
}
func (b FileWriterBuilder) FormatString(format string) FileWriterBuilder {
	b.writer.formatString = format
	return b
}
func (b FileWriterBuilder) New() Writer {
	if b.writer.FilePath == "" {
		b.writer.FilePath = "logs.log"
	}
	if b.writer.formatString == "" {
		b.writer.formatString = "%s"
	}

	if dir := filepath.Dir(b.writer.FilePath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &fileWriter{
				formatString: b.writer.formatString,
				FilePath:     b.writer.FilePath,
				err:          fmt.Errorf("failed to create log directory: %w", err),
			}
		}
	}

	file, err := os.OpenFile(b.writer.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &fileWriter{
			formatString: b.writer.formatString,
			FilePath:     b.writer.FilePath,
			file:         nil,
			err:          fmt.Errorf("failed to open log file: %w", err),
		}
	}

	return &fileWriter{
		formatString: b.writer.formatString,
		FilePath:     b.writer.FilePath,
		file:         file,
		err:          nil,
	}
}

var _ Writer = (*consoleWriter)(nil)
var _ Writer = (*fileWriter)(nil)

var _ WriterBuilder = (*ConsoleWriterBuilder)(nil)
var _ WriterBuilder = (*FileWriterBuilder)(nil)

var ConsoleWriter ConsoleWriterBuilder
var FileWriter FileWriterBuilder
