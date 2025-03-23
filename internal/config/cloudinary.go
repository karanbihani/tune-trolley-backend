package config

import (
	"github.com/cloudinary/cloudinary-go/v2"

	"log"
	"os"
)

var Cloudinary *cloudinary.Cloudinary

func InitCloudinary() {
	CLD_CLOUD_NAME := os.Getenv("CLD_CLOUD_NAME")
	CLD_API_KEY := os.Getenv("CLD_API_KEY")
	CLD_API_SECRET := os.Getenv("CLD_API_SECRET")

	cld, err := cloudinary.NewFromParams(CLD_CLOUD_NAME, CLD_API_KEY, CLD_API_SECRET)
	if err != nil {
		log.Fatalf("Failed to initialize Cloudinary: %v", err)
	}
	Cloudinary = cld
}
