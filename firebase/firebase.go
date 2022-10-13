package firebase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Configures how a new instance of "firebase.google.com/go/v4.App" is created.
type Config struct {
	// If set to true, configures the app to use local firebase emulators.
	// The project id used by the emulators needs to be provided as well, either through this config or the "FIREBASE_PROJECT_ID" environment variable.
	// Note that you will also have to set an environment variable for each firebase component you want to use.
	// E.g. to use the Authentication or Firestore emulators the "FIREBASE_AUTH_EMULATOR_HOST" or "FIRESTORE_EMULATOR_HOST" environment variables
	// need to be set to the respective address the emulator is running on.
	UseEmulators bool
	ProjectId    string
	// Use this service account file to configure the app.
	ServiceAccountFile string
}

// Returns a new App instance that can be used to obtain clients for different firebase components like Firestore or Authentication.
//
// If the provided config is empty, will attempt to create the app using application default credentials.
// These are automatically available when running this code in a Google cloud product like Compute Engine, Cloud Run or App Engine.
// See the comments on the Config type for more information about how to configure the app.
func NewApp(config Config) (*firebase.App, error) {
	if config.UseEmulators {
		projectID := config.ProjectId
		if projectID == "" {
			projectID = os.Getenv("FIREBASE_PROJECT_ID")
			if projectID == "" {
				return nil, errors.New("set FIREBASE_PROJECT_ID environment variable to use local firebase emulators")
			}
		}

		//TODO remove this if no longer necessary
		//this is necessary for auth emulator to work
		//might not be necessary anymore/solved in a different way in a future release
		//see https://github.com/firebase/firebase-admin-go/issues/458
		/*
			authOpt := option.WithTokenSource(
				oauth2.StaticTokenSource(&oauth2.Token{
					AccessToken: "owner",
					TokenType:   "Bearer",
				}),
			)
		*/
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		fbApp, err := firebase.NewApp(ctx, &firebase.Config{
			ProjectID: projectID,
		})
		if err != nil {
			return nil, errors.New(fmt.Sprintf("couldn't init firebase app: %v", err))
		}
		return fbApp, nil
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if config.ServiceAccountFile != "" {
			fbApp, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(config.ServiceAccountFile))
			if err != nil {
				return nil, errors.New(fmt.Sprintf("couldn't init firebase app using service account file: %v", err))
			}
			return fbApp, nil
		} else {
			//attempt use application default credentials
			fbApp, err := firebase.NewApp(ctx, nil)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("couldn't init firebase app using application default credentials: %v", err))
			}
			return fbApp, nil
		}
	}
}

func NewFirestoreClient(app *firebase.App) (*firestore.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		return firestoreClient, errors.New(fmt.Sprintf("couldn't init firestore: %v", err))
	}
	return firestoreClient, nil
}

func NewAuthClient(app *firebase.App) (*auth.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	authClient, err := app.Auth(ctx)
	if err != nil {
		return authClient, errors.New(fmt.Sprintf("couldn't init firebase auth: %v", err))
	}
	return authClient, nil
}
