package models

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	// PostgreSQL connection string format: "user=username password=password dbname=mydb sslmode=disable"
	connection, err := gorm.Open(postgres.Open("host=localhost user=root password=Abishek@15 dbname=go-auth port=5432 sslmode=disable"), &gorm.Config{})

	if err != nil {
		panic("could not connect to the database")
	}

	DB = connection

	connection.AutoMigrate(&models.User{})
}
