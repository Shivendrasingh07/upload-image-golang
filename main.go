package main

import (
	"context"
	"encoding/json"
	"example.com/hello/models"
	"fmt"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/iterator"
	"io"
	"log"
	"net/http"
	"os"

	firebase "firebase.google.com/go"

	"cloud.google.com/go/firestore"
	cloud "cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"google.golang.org/api/option"
)

type App struct {
	Router  *mux.Router
	ctx     context.Context
	client  *firestore.Client
	storage *cloud.Client
}

func main() {
	fmt.Println("test")
	//if loadErr := godotenv.Load(); loadErr != nil {
	//	panic(loadErr)
	//}
	route := App{}
	implicit()
	route.Init()
	route.Run()
}

const projectID = "upload-image-golang"

func implicit() {
	ctx := context.Background()
	key := os.Getenv("FCP_key")
	sa := option.WithCredentialsJSON([]byte(key))
	// For API packages whose import path is starting with "cloud.google.com/go",
	// such as cloud.google.com/go/storage in this case, if there are no credentials
	// provided, the client library will look for credentials in the environment.
	storageClient, err := cloud.NewClient(ctx, sa)
	if err != nil {
		log.Fatal(err)
	}
	defer storageClient.Close()
	fmt.Println("test")

	it := storageClient.Buckets(ctx, projectID)
	for {
		bucketAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(bucketAttrs.Name)
	}

	// For packages whose import path is starting with "google.golang.org/api",
	// such as google.golang.org/api/cloudkms/v1, use NewService to create the client.
	kmsService, err := cloudkms.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	_ = kmsService
}

func (route *App) Init() {
	var err error
	route.ctx = context.Background()
	ctx := context.Background()
	key := os.Getenv("FCP_key")
	sa := option.WithCredentialsJSON([]byte(key))

	app, err := firebase.NewApp(route.ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalln(err)
	}
	client.Collection("gs://upload-image-golang.appspot.com/pilot")

	route.client, err = app.Firestore(route.ctx)
	if err != nil {
		log.Fatalln(err)
	}

	route.storage, err = cloud.NewClient(route.ctx, sa)
	if err != nil {
		log.Fatalln(err)
	}

	route.Router = mux.NewRouter()
	route.initializeRoutes()
	fmt.Println("Successfully connected at port : " + route.GetPort())
}

func (route *App) GetPort() string {
	var port = os.Getenv("MyPort")
	if port == "" {
		port = "5000"
	}
	return ":" + port
}

func (route *App) Run() {
	log.Fatal(http.ListenAndServe(route.GetPort(), route.Router))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		logrus.Errorf("Error encoding response %v", err)
	}

}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func (route *App) initializeRoutes() {
	route.Router.HandleFunc("/", route.Home).Methods("GET")
	route.Router.HandleFunc("/upload/image", route.UploadImage).Methods("POST")
}

func (route *App) Home(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, "Hello World!")
}

func (route *App) UploadImage(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseMultipartForm(51 << 20); err != nil {
		if err == io.EOF || err.Error() == "multipart: NextPart: unexpected EOF" {
			logrus.Warn("EOF")
		} else {
			logrus.Errorf("[ParseMultipartForm] %s", err.Error())
		}
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	file, handler, err := r.FormFile("image")
	defer func() {
		if err = file.Close(); err != nil {
			logrus.Errorf("Unable to close file multipart form. %+v", err)
		}
	}()

	imagePath := handler.Filename

	bucket := "gs://upload-image-golang.appspot.com/pilot"
	fileName := ""

	file, err = os.Open(fileName)
	if err != nil {
		logrus.Errorf("Unable to open file multipart form. %+v", err)
		return
	}
	wc := route.storage.Bucket(bucket).Object(imagePath).NewWriter(route.ctx)
	_, err = io.Copy(wc, file)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return

	}
	if err := wc.Close(); err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	err = CreateImageUrl(imagePath, bucket, route.ctx, route.client)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, "Create image success.")
}

func CreateImageUrl(imagePath string, bucket string, ctx context.Context, client *firestore.Client) error {
	imageStructure := models.ImageStructure{
		ImageName: imagePath,
		URL:       "https://storage.cloud.google.com/" + bucket + "/" + imagePath,
	}

	_, _, err := client.Collection("image").Add(ctx, imageStructure)
	if err != nil {
		return err
	}

	return nil
}
