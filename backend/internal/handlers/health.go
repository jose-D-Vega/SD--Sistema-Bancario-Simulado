package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func Health(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := dbConn.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"db":     "desconectada",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"banco":  bancoID,
		})
	}
}
