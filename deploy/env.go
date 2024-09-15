package deploy

import (
	"github.com/samber/lo"
	"log"
	"os"
)

var ENV, _ = lo.Coalesce(os.Getenv("ENV"), DevENV)

const (
	DevENV     = "dev"
	ProdENV    = "prod"
	StagingEnv = "staging"
)

func IsReleaseEnv() bool {
	return ENV == ProdENV || ENV == StagingEnv
}

// Prevent use of production values in development.
func init() {
	if ENV != DevENV && ENV != ProdENV && ENV != StagingEnv {
		log.Fatalf("unrecognized value for variable variable 'ENV': '%s'", ENV)
	}
}
