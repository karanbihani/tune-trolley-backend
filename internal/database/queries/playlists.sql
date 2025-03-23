-- name: CreatePlaylist :one
INSERT INTO playlists (playlist_id, user_uuid, name, playlist_code, image_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPlaylists :many
SELECT *
FROM playlists
WHERE user_uuid = $1;

-- name: GetPlaylist :one
SELECT *
FROM playlists
WHERE playlist_uuid = $1;

-- name: GetPlaylistUUIDByName :one
SELECT playlist_uuid
FROM playlists
WHERE user_uuid = $1 AND name = $2;

-- name: GetPlaylistIDByUUID :one
SELECT playlist_id
FROM playlists
WHERE playlist_uuid = $1;

-- name: GetPlaylistUUIDByCode :one
SELECT playlist_uuid
FROM playlists
WHERE playlist_code = $1;

-- name: UpdatePlaylistName :one
UPDATE playlists
SET name = $1
WHERE playlist_uuid = $2
RETURNING *;

-- name: DeletePlaylist :execrows
DELETE FROM playlists
WHERE playlist_uuid = $1;

-- name: ListOwnedPlaylists :many
SELECT p.*, COUNT(pm2.user_uuid) AS member_count
FROM playlists p
JOIN playlist_members pm ON p.playlist_uuid = pm.playlist_uuid
LEFT JOIN playlist_members pm2 ON p.playlist_uuid = pm2.playlist_uuid
WHERE pm.user_uuid = $1 AND pm.role = 'owner'
GROUP BY p.playlist_uuid;

-- name: ListMemberPlaylists :many
SELECT p.*, COUNT(pm2.user_uuid) AS member_count
FROM playlists p
JOIN playlist_members pm ON p.playlist_uuid = pm.playlist_uuid
LEFT JOIN playlist_members pm2 ON p.playlist_uuid = pm2.playlist_uuid
WHERE pm.user_uuid = $1 AND pm.role = 'member'
GROUP BY p.playlist_uuid;

-- name: GetPlaylistOwner :one
SELECT user_uuid
FROM playlists
WHERE playlist_uuid = $1;