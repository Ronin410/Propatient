#FROM golang:1.25-alpine
#RUN apk add --no-cache git
#WORKDIR /app
# Instalamos Air para que el código de Go se reinicie solo al guardar
#RUN go install github.com/air-verse/air@v1.52.3
#COPY go.mod go.sum ./
#RUN go mod download
#COPY . .
#EXPOSE 8095
#CMD ["air", "-c", ".air.toml"]

FROM golang:1.25-alpine

WORKDIR /app

# Copiamos go.mod/go.sum primero para aprovechar la cache de capas de Docker:
# mientras no cambien las dependencias, "go mod download" no se vuelve a
# ejecutar en builds siguientes.
COPY go.mod go.sum ./
RUN go mod download

# Copiamos el resto del código fuente
COPY . .

# La carpeta "vendor/" nunca se subió al repo (está en .gitignore), así que
# no podemos compilar con -mod=vendor aquí: usamos el modo normal de
# módulos, con las dependencias ya descargadas en la capa anterior.
RUN go build -o main .

EXPOSE 8095

CMD ["./main"]