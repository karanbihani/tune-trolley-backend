package songs

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func validateAddSongToDBReq(c *gin.Context) (AddSongToDBReq, error) {
	var req AddSongToDBReq
	err := c.ShouldBindJSON(&req)
	return req, err
}
func validateAddSongToPlaylistReq(c *gin.Context) (AddSongToPlaylistReq, error) {
	var req AddSongToPlaylistReq
	err := c.ShouldBindBodyWithJSON(&req)
	if err != nil {
		return req, err
	}
	err = c.ShouldBindUri(&req)
	if req.Option != "reject" && req.Option != "accept" {
		return req, errors.New("should be either accpet or reject(case sensitive)")
	}
	return req, err
}

func validateGetAllSongsReq(c *gin.Context) (GetAllSongsReq, error) {
	var req GetAllSongsReq
	err := c.ShouldBindUri(&req)
	return req, err
}

func validateBlacklistSongReq(c *gin.Context) (BlacklistSongReq, error) {
	var req BlacklistSongReq
	err := c.ShouldBindJSON(&req)
	return req, err
}

func validateDeleteSongReq(c *gin.Context) (DeleteSongReq, error) {
	var req DeleteSongReq
	err := c.ShouldBindJSON(&req)
	return req, err
}
