package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv" // Cargar variables desde el archivo .env

	_ "github.com/go-kivik/couchdb/v3" // Importar el driver de CouchDB
	"github.com/go-kivik/kivik/v3"
)

func main() {
	// Cargar el archivo .env
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error al cargar el archivo .env: %v", err)
	}

	// Obtener valores del archivo .env
	couchDBURL := os.Getenv("COUCHDB")
	serverPort := os.Getenv("PORT")
	if couchDBURL == "" || serverPort == "" {
		log.Fatal("Las variables COUCHDB y PORT son obligatorias en el archivo .env")
	}

	// Crear un contexto de fondo
	ctx := context.Background()

	// Conectar a CouchDB
	client, err := kivik.New("couch", couchDBURL)
	if err != nil {
		log.Fatalf("Error al conectar con CouchDB: %v", err)
	}

	if err := createDatabase(ctx, client, "_users"); err != nil {
		log.Fatalf("Error al crear la base de datos 'prograred': %v", err)
	}

	// Crear la base de datos
	dbName := "prograred"
	if err := createDatabase(ctx, client, dbName); err != nil {
		log.Fatalf("Error al crear la base de datos: %v", err)
	}

	// Configurar las rutas del servidor
	http.HandleFunc("/api/couch_db", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			id := r.URL.Query().Get("id")
			if id == "" {
				getAllDocuments(w, r, client, dbName)
			} else {
				getDocumentByID(w, r, client, dbName, id)
			}
		case "POST":
			insertDocument(w, r, client, dbName)
		case "PUT":
			updateDocumentByID(w, r, client, dbName)
		case "DELETE":
			deleteDocumentByID(w, r, client, dbName)
		default:
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		}
	})

	// Iniciar el servidor en el puerto especificado
	fmt.Printf("Servidor escuchando en el puerto %s...\n", serverPort)
	log.Fatal(http.ListenAndServe(":"+serverPort, nil))
}

// Función para crear la base de datos
func createDatabase(ctx context.Context, client *kivik.Client, dbName string) error {
	exists, err := client.DBExists(ctx, dbName)
	if err != nil {
		return err
	}
	if exists {
		fmt.Println("La base de datos ya existe.")
		return nil
	}
	return client.CreateDB(ctx, dbName)
}

// Función para insertar un documento
func insertDocument(w http.ResponseWriter, r *http.Request, client *kivik.Client, dbName string) {
	var request struct {
		ID       string                 `json:"id"`
		Document map[string]interface{} `json:"document"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Error al leer el documento", http.StatusBadRequest)
		return
	}

	if request.ID == "" {
		http.Error(w, "El campo 'id' es requerido", http.StatusBadRequest)
		return
	}

	db := client.DB(r.Context(), dbName)
	_, err := db.Put(r.Context(), request.ID, request.Document)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al insertar documento: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Documento insertado con ID: %s\n", request.ID)
}

// Función para obtener todos los documentos
func getAllDocuments(w http.ResponseWriter, r *http.Request, client *kivik.Client, dbName string) {
	db := client.DB(r.Context(), dbName)
	rows, err := db.AllDocs(r.Context(), map[string]interface{}{"include_docs": true})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al obtener documentos: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var documents []map[string]interface{}
	for rows.Next() {
		var doc map[string]interface{}
		if err := rows.ScanDoc(&doc); err != nil {
			http.Error(w, fmt.Sprintf("Error al leer un documento: %v", err), http.StatusInternalServerError)
			return
		}
		documents = append(documents, doc)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Error al procesar los documentos: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(documents); err != nil {
		http.Error(w, fmt.Sprintf("Error al codificar la respuesta JSON: %v", err), http.StatusInternalServerError)
	}
}

// Función para obtener un documento por ID
func getDocumentByID(w http.ResponseWriter, r *http.Request, client *kivik.Client, dbName, id string) {
	db := client.DB(r.Context(), dbName)
	doc := map[string]interface{}{}
	if err := db.Get(r.Context(), id).ScanDoc(&doc); err != nil {
		http.Error(w, fmt.Sprintf("Error al obtener el documento: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		http.Error(w, fmt.Sprintf("Error al codificar la respuesta JSON: %v", err), http.StatusInternalServerError)
	}
}

// Función para actualizar un documento por ID
func updateDocumentByID(w http.ResponseWriter, r *http.Request, client *kivik.Client, dbName string) {
	// Parsear la solicitud para obtener el ID y el documento
	var request struct {
		ID       string                 `json:"id"`
		Document map[string]interface{} `json:"document"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Error al leer el cuerpo de la solicitud", http.StatusBadRequest)
		return
	}

	// Validar el ID del documento
	if request.ID == "" {
		http.Error(w, "El campo 'id' es requerido", http.StatusBadRequest)
		return
	}

	// Conectarse a la base de datos
	db := client.DB(r.Context(), dbName)

	// Obtener el documento actual para recuperar su _rev
	doc := db.Get(r.Context(), request.ID)
	var currentDoc map[string]interface{}
	if err := doc.ScanDoc(&currentDoc); err != nil {
		http.Error(w, fmt.Sprintf("Error al obtener el documento: %v", err), http.StatusNotFound)
		return
	}

	// Verificar si el documento contiene _rev
	rev, ok := currentDoc["_rev"].(string)
	if !ok {
		http.Error(w, "No se pudo obtener la revisión del documento", http.StatusInternalServerError)
		return
	}

	// Incluir _rev en el documento a actualizar
	request.Document["_rev"] = rev

	// Actualizar el documento
	_, err := db.Put(r.Context(), request.ID, request.Document)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al actualizar documento: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Documento con ID %s actualizado exitosamente\n", request.ID)
}

// Función para eliminar un documento por ID
func deleteDocumentByID(w http.ResponseWriter, r *http.Request, client *kivik.Client, dbName string) {
	// Parsear el ID del documento desde la URL
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "El campo 'id' es requerido", http.StatusBadRequest)
		return
	}

	// Conectar a la base de datos
	db := client.DB(r.Context(), dbName)

	// Obtener el documento actual para recuperar su _rev
	doc := db.Get(r.Context(), id)
	var currentDoc map[string]interface{}
	if err := doc.ScanDoc(&currentDoc); err != nil {
		http.Error(w, fmt.Sprintf("Error al obtener el documento: %v", err), http.StatusNotFound)
		return
	}

	// Verificar si el documento contiene _rev
	rev, ok := currentDoc["_rev"].(string)
	if !ok {
		http.Error(w, "No se pudo obtener la revisión del documento", http.StatusInternalServerError)
		return
	}

	// Eliminar el documento usando el ID y _rev
	if _, err := db.Delete(r.Context(), id, rev); err != nil {
		http.Error(w, fmt.Sprintf("Error al eliminar el documento: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Documento con ID %s eliminado exitosamente\n", id)
}
