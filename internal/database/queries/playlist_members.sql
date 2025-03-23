-- name: GetPlaylistMember :one
SELECT user_uuid, playlist_uuid, role, joined_at, updated_at
FROM playlist_members
WHERE user_uuid = $1 AND playlist_uuid = $2;

-- name: AddPlaylistMember :exec
INSERT INTO playlist_members (user_uuid, playlist_uuid, role, joined_at, updated_at)
VALUES ($1, $2, $3, now(), now());