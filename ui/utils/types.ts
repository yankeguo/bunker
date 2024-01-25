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
  is_admin: string;
}

export interface BToken {
  id: string;
  user_id: string;
  user_agent: string;
  created_at: string;
  visited_at: string;
}
