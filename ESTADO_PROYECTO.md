# Estado del Proyecto `ATV-Backend`

---

## 🟢 Funcionalidades Implementadas

- **API RESTful para gestión de estudiantes:**
  - Endpoints CRUD completos para estudiantes (`/api/vX/student`, `/api/vX/students`).
  - Soporte para soft delete y borrado permanente.
  - Filtros por ID, número de documento, email, y estado de borrado.
- **Arquitectura modular:**
  - Separación clara en controladores, servicios, DAOs, modelos y utilidades.
  - Uso de Gin como framework HTTP.
- **Configuración flexible:**
  - Variables de entorno para modo, versión, dirección y base de datos.
  - Soporte para múltiples entornos (dev, prod, debug, test).
- **Persistencia:**
  - Acceso a base de datos MongoDB (accesor definido y configurable).
  - Migraciones automáticas para modelos.
- **Contenedores y despliegue:**
  - Dockerfile y docker-compose listos para desarrollo y producción.
  - Configuración para despliegue en Cloud Run y Skaffold.
- **Logging y manejo de errores:**
  - Logging estructurado y detallado en cada capa.
  - Manejo de errores HTTP consistente y mensajes claros.

---

## 🟡 Funcionalidades Parcialmente Implementadas o Frágiles

- **Versionado de rutas:**
  - Soporte para múltiples versiones de API, pero rutas antiguas solo redirigen o están deprecadas.
- **Validaciones:**
  - Validación básica de datos de entrada, pero sin validaciones avanzadas (regex, unicidad, etc).
- **Manejo de relaciones:**
  - IDs de universidad y otras entidades se validan solo como strings/DBID, no se verifica existencia real.

---

## 🔴 Funcionalidades Faltantes o Incompletas

- **Autenticación y autorización:**
  - Middleware de token comentado, no hay control de acceso real.
  - No hay roles ni permisos implementados.
- **Documentación de endpoints:**
  - No hay Swagger/OpenAPI ni documentación detallada de la API.
- **Pruebas automatizadas:**
  - No se detectan tests unitarios ni de integración.
- **Alertas, prioridades, acompañamientos, formularios, etc:**
  - Estructura de rutas y servicios para estas entidades, pero falta revisar implementación completa (solo se analizó a fondo estudiantes).
- **Manejo avanzado de errores de base de datos:**
  - Errores de conexión y migración se loguean pero no se recuperan automáticamente.

---

## ⚠️ Problemas Detectados y Riesgos

- **Dependencia de IDs externos:**
  - El frontend depende de IDs fijos de universidades, tipos, etc. El backend no expone endpoints para listarlos fácilmente.
- **Validación superficial:**
  - Si el frontend envía datos mal formateados, la validación es mínima y puede guardar datos inconsistentes.
- **Falta de autenticación:**
  - Cualquier usuario puede acceder a los endpoints si el middleware no está activo.
- **Migraciones automáticas:**
  - Si el modelo cambia y hay datos previos, puede haber inconsistencias.

---

## 🧩 Gaps Universales

- Falta de autenticación y autorización real.
- Falta de documentación de la API.
- Sin pruebas automatizadas.
- No hay endpoints para obtener IDs de entidades relacionadas (universidades, tipos, etc).
- Validación de datos limitada.

---

## 📋 Siguientes pasos sugeridos

- Activar y mejorar el middleware de autenticación.
- Implementar roles y permisos básicos.
- Añadir documentación Swagger/OpenAPI.
- Crear endpoints para listar IDs y entidades relacionadas.
- Mejorar validaciones de entrada.
- Añadir pruebas unitarias y de integración.

---

## ❓ Preguntas para el equipo

- ¿Qué entidades deben tener endpoints públicos para listar IDs?
- ¿Qué nivel de autenticación y roles se requiere?
- ¿Se necesita soporte para otras bases de datos además de MongoDB?
- ¿Quieren priorizar la documentación o la seguridad primero?

---

> _Colores:_
> - 🟢 Implementado
> - 🟡 Parcial
> - 🔴 Faltante
> - ⚠️ Problema
> - 🧩 Gap universal
> - 📋 Siguiente paso
> - ❓ Pregunta

---
