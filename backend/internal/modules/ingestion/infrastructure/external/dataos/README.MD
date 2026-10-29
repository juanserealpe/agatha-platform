# DataOS Adapter

Adapter encargado de integrar el módulo de Ingestion de AGATHA con la plataforma externa DataOS.

## Responsabilidad

El adapter encapsula los detalles de comunicación con DataOS actualmente mientras se implementa el CallBack que también va a tener su adapter a parte, en DataOS maneja:

- Autenticación.
- Manejo de sesión mediante cookies.
- Consulta de datos de dispositivos.
- Transformación de respuestas externas al modelo interno de AGATHA.

El resto del módulo de Ingestion no debe depender directamente de endpoints, cookies,
tokens ni estructuras JSON propias de DataOS.

## Estructura Actual

dataos/
├── auth/
│   ├── client.go
│   └── models.go
│
├── dashboard/
│   ├── client.go
│   └── models.go
│
├── mapper.go
├── client.go
└── README.md