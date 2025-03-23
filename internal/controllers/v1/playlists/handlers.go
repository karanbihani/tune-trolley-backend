package playlists

import (
	"errors"
	"fmt"
	"net/http"
	"spotify-collab/internal/config"
	"spotify-collab/internal/controllers/v1/auth"
	"spotify-collab/internal/database"
	"spotify-collab/internal/merrors"
	"spotify-collab/internal/utils"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type PlaylistHandler struct {
	db          *pgxpool.Pool
	spotifyauth *spotifyauth.Authenticator
}

func Handler(db *pgxpool.Pool, spotifyAuth *spotifyauth.Authenticator) *PlaylistHandler {
	return &PlaylistHandler{
		db:          db,
		spotifyauth: spotifyAuth,
	}
}

func (p *PlaylistHandler) CreatePlaylist(c *gin.Context) {
	req, err := validateCreatePlaylist(c)
	if err != nil {
		merrors.Validation(c, err.Error())
		return
	}

	u, ok := c.Get("user")
	if !ok {
		panic("user failed to set in context")
	}
	user := u.(*auth.ContextUser)
	if user == auth.AnonymousUser {
		merrors.Unauthorized(c, "This action is forbidden.")
		return
	}

	tx, err := p.db.Begin(c)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}
	defer tx.Rollback(c)
	qtx := database.New(p.db).WithTx(tx)

	token, err := qtx.GetOAuthToken(c, user.UserUUID)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	oauthToken := &oauth2.Token{
		AccessToken:  string(token.Access),
		RefreshToken: string(token.Refresh),
		Expiry:       token.Expiry.Time,
	}

	tokenChanged := false
	if !oauthToken.Valid() {
		oauthToken, err = p.spotifyauth.RefreshToken(c, oauthToken)
		tokenChanged = true
		if err != nil {
			merrors.InternalServer(c, fmt.Sprintf("Couldn't get access token %s", err))
			return
		}

		_, err := qtx.UpdateToken(c, database.UpdateTokenParams{
			Refresh:  []byte(oauthToken.RefreshToken),
			Access:   []byte(oauthToken.AccessToken),
			Expiry:   pgtype.Timestamptz{Time: oauthToken.Expiry, Valid: true},
			UserUuid: user.UserUUID,
		})
		if err != nil {
			merrors.InternalServer(c, err.Error())
			return
		}
	}

	var imageURL string
	file, _, err := c.Request.FormFile("image")
	if err == nil {
		defer file.Close()
		uploadResult, err := config.Cloudinary.Upload.Upload(c, file, uploader.UploadParams{
			Folder:         "collabify/playlists",
			Transformation: "c_fill,g_auto,h_250,w_250",
			PublicID:       fmt.Sprintf("%s-%s", user.UserUUID, req.Name),
			Tags:           []string{"collabify", "playlist", "image"},
			Context:        map[string]string{"user_uuid": user.UserUUID.String(), "playlist_name": req.Name},
		})
		if err != nil {
			merrors.InternalServer(c, fmt.Sprintf("Failed to upload image: %s", err.Error()))
			return
		}
		imageURL = uploadResult.SecureURL
	}

	// Generate Playlist from spotify
	client := spotify.New(p.spotifyauth.Client(c, oauthToken))
	spotifyPlaylist, err := client.CreatePlaylistForUser(c, token.SpotifyID, req.Name, "", true, false)
	if err != nil {
		merrors.InternalServer(c, fmt.Sprintf("Error while creating spotify playlist: %s", err.Error()))
		return
	}

	playlist, err := qtx.CreatePlaylist(c, database.CreatePlaylistParams{
		PlaylistID:   spotifyPlaylist.ID.String(),
		UserUuid:     user.UserUUID,
		Name:         req.Name,
		PlaylistCode: GeneratePlaylistCode(6),
		ImageUrl:     &imageURL,
	})
	// Check name already exists
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	// Add user as owner to playlist_members
	err = qtx.AddPlaylistMember(c, database.AddPlaylistMemberParams{
		UserUuid:     user.UserUUID,
		PlaylistUuid: playlist.PlaylistUuid,
		Role:         "owner",
	})
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	err = tx.Commit(c)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	if tokenChanged {
		c.JSON(http.StatusOK, utils.BaseResponse{
			Success:    true,
			Message:    "playlist successfully created",
			Data:       playlist,
			MetaData:   oauthToken,
			StatusCode: http.StatusOK,
		})
	} else {
		c.JSON(http.StatusOK, utils.BaseResponse{
			Success:    true,
			Message:    "playlist successfully created",
			Data:       playlist,
			StatusCode: http.StatusOK,
		})
	}
}

func (p *PlaylistHandler) JoinPlaylist(c *gin.Context) {
	req, err := validateJoinPlaylistReq(c)
	if err != nil {
		merrors.Validation(c, err.Error())
		return
	}

	u, ok := c.Get("user")
	if !ok {
		panic(" user failed to set in context ")
	}
	user := u.(*auth.ContextUser)
	if user == auth.AnonymousUser {
		merrors.Unauthorized(c, "This action is forbidden.")
		return
	}

	tx, err := p.db.Begin(c)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback(c)
		}
	}()

	qtx := database.New(p.db).WithTx(tx)

	playlist, err := qtx.GetPlaylistUUIDByCode(c, req.PlaylistCode)
	if errors.Is(err, pgx.ErrNoRows) {
		merrors.NotFound(c, "no playlist found")
		return
	} else if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	// Check if user is already a member of the playlist
	_, err = qtx.GetPlaylistMember(c, database.GetPlaylistMemberParams{
		UserUuid:     user.UserUUID,
		PlaylistUuid: playlist,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Determine the role of the user (owner or member)
		role := "member"
		playlistOwner, err := qtx.GetPlaylistOwner(c, playlist)
		if err == nil && playlistOwner == user.UserUUID {
			role = "owner"
		}

		// Add user as a member of the playlist
		err = qtx.AddPlaylistMember(c, database.AddPlaylistMemberParams{
			UserUuid:     user.UserUUID,
			PlaylistUuid: playlist,
			Role:         role,
		})
		if err != nil {
			merrors.InternalServer(c, err.Error())
			return
		}
	} else if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	} else {
		// User is already a member
		c.JSON(http.StatusOK, utils.BaseResponse{
			Success:    true,
			Message:    "User is already a member of the playlist",
			StatusCode: http.StatusOK,
		})
		return
	}

	token, err := qtx.GetOAuthToken(c, user.UserUUID)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	oauthToken := &oauth2.Token{
		AccessToken:  string(token.Access),
		RefreshToken: string(token.Refresh),
		Expiry:       token.Expiry.Time,
	}

	tokenChanged := false
	if !oauthToken.Valid() {
		oauthToken, err = p.spotifyauth.RefreshToken(c, oauthToken)
		tokenChanged = true
		if err != nil {
			merrors.InternalServer(c, fmt.Sprintf("Couldn't get access token %s", err))
			return
		}

		_, err := qtx.UpdateToken(c, database.UpdateTokenParams{
			Refresh:  []byte(oauthToken.RefreshToken),
			Access:   []byte(oauthToken.AccessToken),
			Expiry:   pgtype.Timestamptz{Time: oauthToken.Expiry, Valid: true},
			UserUuid: user.UserUUID,
		})
		if err != nil {
			merrors.InternalServer(c, err.Error())
			return
		}
	}

	err = tx.Commit(c)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	if tokenChanged {
		c.JSON(http.StatusOK, utils.BaseResponse{
			Success:    true,
			Message:    "Successfully joined playlist",
			MetaData:   oauthToken,
			StatusCode: http.StatusOK,
		})
	} else {
		c.JSON(http.StatusOK, utils.BaseResponse{
			Success:    true,
			Message:    "Successfully joined playlist",
			StatusCode: http.StatusOK,
		})
	}
}

func (p *PlaylistHandler) ListPlaylists(c *gin.Context) {
	u, ok := c.Get("user")
	if !ok {
		panic(" user failed to set in context ")
	}
	user := u.(*auth.ContextUser)
	if user == auth.AnonymousUser {
		merrors.Unauthorized(c, "This action is forbidden.")
		return
	}

	q := database.New(p.db)

	ownedPlaylists, err := q.ListOwnedPlaylists(c, user.UserUUID)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	memberPlaylists, err := q.ListMemberPlaylists(c, user.UserUUID)
	if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	if len(ownedPlaylists) == 0 && len(memberPlaylists) == 0 {
		merrors.NotFound(c, "No Playlists exist!")
		return
	}

	c.JSON(http.StatusOK, utils.BaseResponse{
		Success: true,
		Message: "Playlists successfully retrieved",
		Data: map[string]interface{}{
			"owner":  ownedPlaylists,
			"member": memberPlaylists,
		},
		StatusCode: http.StatusOK,
	})
}

func (p *PlaylistHandler) GetPlaylist(c *gin.Context) {
	req, err := validateGetPlaylistReq(c)
	if err != nil {
		merrors.Validation(c, err.Error())
		return
	}
	// No need for check err since binding checks uuid
	uuid, _ := uuid.Parse(req.PlaylistUUID)

	q := database.New(p.db)
	playlist, err := q.GetPlaylist(c, uuid)
	if errors.Is(err, pgx.ErrNoRows) {
		merrors.NotFound(c, "Playlist not found")
		return
	} else if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, utils.BaseResponse{
		Success:    true,
		Message:    "Playlist successfully retrieved",
		Data:       playlist,
		StatusCode: http.StatusOK,
	})
}

func (p *PlaylistHandler) UpdatePlaylist(c *gin.Context) {
	req, err := validateUpdatePlaylistReq(c)
	if err != nil {
		merrors.Validation(c, err.Error())
		return
	}
	// No need for check err since binding checks uuid
	uuid, _ := uuid.Parse(req.PlaylistUUID)

	q := database.New(p.db)
	playlist, err := q.UpdatePlaylistName(c, database.UpdatePlaylistNameParams{
		Name:         req.Name,
		PlaylistUuid: uuid,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		merrors.NotFound(c, "Playlist not found")
		return
	} else if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, utils.BaseResponse{
		Success:    true,
		Message:    "Playlist successfully updated",
		Data:       playlist,
		StatusCode: http.StatusOK,
	})
}

func (p *PlaylistHandler) DeletePlaylist(c *gin.Context) {
	req, err := validateDeletePlaylistReq(c)
	if err != nil {
		merrors.Validation(c, err.Error())
		return
	}
	// No need for check err since binding checks uuid
	uuid, _ := uuid.Parse(req.PlaylistUUID)

	q := database.New(p.db)
	rows, err := q.DeletePlaylist(c, uuid)
	if rows == 0 {
		merrors.NotFound(c, "Playlist not found")
		return
	} else if err != nil {
		merrors.InternalServer(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, utils.BaseResponse{
		Success:    true,
		Message:    "Playlist successfully deleted",
		StatusCode: http.StatusOK,
	})
}
