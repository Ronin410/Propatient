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

# Copiamos todo tu código incluyendo la nueva carpeta 'vendor'
COPY . .

# Compilamos usando la carpeta vendor local (-mod=vendor)
# ¡Cero peticiones a internet, cero errores de certificados!
RUN go build -mod=vendor -o main .

EXPOSE 8095

CMD ["./main"]