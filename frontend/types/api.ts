// Code generated by tygo. DO NOT EDIT.
import * as node from "./node"

//////////
// source: node.go

export interface MigrateRequest {
  target_version: string;
}
export interface GetNodeResponse {
  state: node.State;
}
export interface PostAddUserRequest {
  user_id: string;
  handle: string;
  password: string;
  email: string;
  certificate: string;
}
export interface PostAddUserResponse {
  pds_create_account_response: { [key: string]: any};
}

//////////
// source: pds.go

export interface PDSCreateAccountRequest {
  email: string;
  handle: string;
  password: string;
}
export type PDSCreateAccountResponse = { [key: string]: any};
export interface PDSCreateSessionRequest {
  identifier: string; // email or handle
  password: string;
}
export type PDSCreateSessionResponse = { [key: string]: any};
