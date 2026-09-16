package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"edunova-server/config"
	"edunova-server/database"
	"edunova-server/routes"
)

func main() {
	config.Load()
	database.Connect()
	defer database.Close()

	r := gin.Default()
	routes.SetupRoutes(r)

	addr := fmt.Sprintf(":%s", config.AppConfig.Port)
	log.Printf("Server running on port %s", config.AppConfig.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
