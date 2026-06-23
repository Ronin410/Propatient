select * from  appointments limit 10;

select * from  doctor_patients limit 10;

select * from  doctors limit 10;

select * from  medical_histories limit 10;

select * from  patients limit 10;


# Crear la red interna
podman network create propatient-net

# Crear el volumen para Postgres
podman volume create postgres_data

podman run -d --name propatient-db --network propatient-net -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=password123 -e POSTGRES_DB=propatient -p 5432:5432 -v postgres_data:/var/lib/postgresql/data postgres:15-alpine

podman build -t propatient-backend .

podman run -d --name propatient-api --network propatient-net -p 8095:8095 -e DATABASE_URL=postgres://postgres:password123@propatient-db:5432/propatient?sslmode=disable -e JWT_SECRET=MiClaveSuperSecreta123 -e PORT=8095 -v .:/app:Z propatient-backend
podman run -d --name propatient-api --network propatient-net -p 8095:8095 -e DATABASE_URL=postgres://postgres:password123@propatient-db:5432/propatient?sslmode=disable -e JWT_SECRET=MiClaveSuperSecreta123 -e PORT=8095 propatient-backend

podman build -t propatient-frontend .

podman run -d --name propatient-web --network propatient-net -p 5173:5173 -e VITE_API_URL=http://localhost:8095/api -v .:/app:Z -v /app/node_modules propatient-frontend

podman start propatient-db
podman start propatient-api
podman start propatient-web

