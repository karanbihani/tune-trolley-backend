package playlists

type CreatePlaylistReq struct {
	Name string `form:"name" binding:"required"`
}

type GetPlaylistReq struct {
	PlaylistUUID string `json:"playlist_uuid" binding:"required,uuid" uri:"id"`
}

type UpdatePlaylistReq struct {
	PlaylistUUID string `binding:"required,uuid" uri:"id"`
	Name         string `json:"name" binding:"required"`
}

type DeletePlaylistReq struct {
	PlaylistUUID string `json:"playlist_uuid" binding:"required,uuid" uri:"id"`
}

type JoinPlaylistReq struct {
	PlaylistCode string `json:"playlist_code" binding:"required"`
}
