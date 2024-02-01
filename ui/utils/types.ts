export interface BServer {
  id: string;
  address: string;
}

export interface BKey {
  id: string;
  display_name: string;
  created_at: string;
  visited_at: string;
}

export interface BUser {
  id: string;
  created_at: string;
  visited_at: string;
  is_admin: boolean;
  is_blocked: boolean;
}

export interface BToken {
  id: string;
  user_id: string;
  user_agent: string;
  created_at: string;
  visited_at: string;
}

export interface BGrant {
  id: string;
  user_id: string;
  server_user: string;
  server_id: string;
  created_at: string;
}