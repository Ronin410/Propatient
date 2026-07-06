select * from  appointments order by appointment_date_time desc  limit 100;

select * from  appointments where status = 'PENDING' AND DOCTOR_ID = 3  limit 100;


select * from  appointments where doctor_id = 3 AND appointment_date_time >= '2026-07-05T07:52:25-07:00' AND appointment_date_time < '2026-07-05T023:52:25-07:00'

select * from patients where id = 42

UPDATE appointments set status = 'COMPLETED' WHERE ID = 20

select * from  doctor_patients limit 10;

select * from  doctors limit 100;

select * from  patients limit 100;


select * from  medical_histories limit 10;

select * from  medical_documents limit 10;

update doctor_patients set doctor_id = 2 where patient_id = 1;

update patients set first_name ='Alejandra', last_name = 'Ordoñez Zuluaga' where id = 3

UPDATE doctors SET cedula_validated = 'VALIDADA' WHERE id = 3


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


wsl --shutdown


podman rm -f propatient-web