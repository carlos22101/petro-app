package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Medicion struct {
	ID          int       `json:"id"`
	Temperatura float64   `json:"temperatura"`
	Presion     float64   `json:"presion"`
	Fecha       string    `json:"fecha"`
	Hora        string    `json:"hora"`
	CreatedAt   time.Time `json:"created_at"`
}

type MedicionRequest struct {
	Temperatura float64 `json:"temperatura"`
	Presion     float64 `json:"presion"`
	Fecha       string  `json:"fecha"`
	Hora        string  `json:"hora"`
}

var db *sql.DB

func initDB() {
	var err error
	// Ajusta estos valores según tu configuración de MySQL
	dsn := "root:233429up@tcp(localhost:3306)/sensor_monitoring?parseTime=true"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Error conectando a la base de datos:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Error al hacer ping a la base de datos:", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Conexión a la base de datos establecida")
}

// CORS middleware
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// POST /api/medicion - Recibe datos de la Raspberry Pi
func crearMedicion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req MedicionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error al decodificar JSON", http.StatusBadRequest)
		return
	}

	query := `INSERT INTO mediciones (temperatura, presion, fecha, hora) VALUES (?, ?, ?, ?)`
	result, err := db.Exec(query, req.Temperatura, req.Presion, req.Fecha, req.Hora)
	if err != nil {
		log.Println("Error al insertar medición:", err)
		http.Error(w, "Error al guardar medición", http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
		"message": "Medición guardada correctamente",
	})
}

// GET /api/ultima-medicion - Obtiene la última medición (para short polling)
func obtenerUltimaMedicion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var medicion Medicion
	query := `SELECT id, temperatura, presion, fecha, hora, created_at 
	          FROM mediciones 
	          ORDER BY created_at DESC 
	          LIMIT 1`

	err := db.QueryRow(query).Scan(
		&medicion.ID,
		&medicion.Temperatura,
		&medicion.Presion,
		&medicion.Fecha,
		&medicion.Hora,
		&medicion.CreatedAt,
	)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "No hay mediciones disponibles",
		})
		return
	}

	if err != nil {
		log.Println("Error al obtener última medición:", err)
		http.Error(w, "Error al obtener medición", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    medicion,
	})
}

// GET /api/mediciones - Obtiene todas las mediciones históricas
func obtenerMediciones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	query := `SELECT id, temperatura, presion, fecha, hora, created_at 
	          FROM mediciones 
	          ORDER BY created_at DESC 
	          LIMIT 100`

	rows, err := db.Query(query)
	if err != nil {
		log.Println("Error al obtener mediciones:", err)
		http.Error(w, "Error al obtener mediciones", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var mediciones []Medicion
	for rows.Next() {
		var m Medicion
		err := rows.Scan(&m.ID, &m.Temperatura, &m.Presion, &m.Fecha, &m.Hora, &m.CreatedAt)
		if err != nil {
			log.Println("Error al escanear medición:", err)
			continue
		}
		mediciones = append(mediciones, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    mediciones,
		"total":   len(mediciones),
	})
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/api/medicion", enableCORS(crearMedicion))
	http.HandleFunc("/api/ultima-medicion", enableCORS(obtenerUltimaMedicion))
	http.HandleFunc("/api/mediciones", enableCORS(obtenerMediciones))

	port := ":8080"
	fmt.Printf("Servidor corriendo en http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
