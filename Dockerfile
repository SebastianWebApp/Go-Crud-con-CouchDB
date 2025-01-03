# Etapa 1: Construcción
FROM golang:1.20-alpine as builder

# Establecer el directorio de trabajo
WORKDIR /

# Copiar los archivos go.mod y go.sum para instalar dependencias
COPY go.mod go.sum ./

# Descargar las dependencias
RUN go mod download

# Copiar el código fuente
COPY . .

# Descargar las dependencias de Go
RUN go mod tidy


# Construir el binario de la aplicación
RUN go build -o app .

# Etapa 2: Imagen final
FROM debian:bullseye-slim

# Establecer el directorio de trabajo
WORKDIR /

# Copiar el binario de la etapa de construcción
COPY --from=builder /app .
COPY --from=builder /.env .



# Exponer el puerto configurado en la variable de entorno
EXPOSE 4005

# Comando de inicio
CMD ["./app"]
