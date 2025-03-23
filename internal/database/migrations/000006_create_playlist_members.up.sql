CREATE TABLE public.playlist_members (
  user_uuid uuid NOT NULL,
  playlist_uuid uuid NOT NULL,
  role text NOT NULL DEFAULT 'member',
  joined_at timestamp with time zone DEFAULT now() NOT NULL,
  updated_at timestamp with time zone DEFAULT now() NOT NULL,
  CONSTRAINT playlist_members_pk PRIMARY KEY (user_uuid, playlist_uuid),
  CONSTRAINT playlist_members_users_fk 
    FOREIGN KEY (user_uuid) 
    REFERENCES public.users(user_uuid) 
    ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT playlist_members_playlists_fk 
    FOREIGN KEY (playlist_uuid) 
    REFERENCES public.playlists(playlist_uuid) 
    ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT playlist_members_role_check 
    CHECK (role IN ('owner', 'member'))
);