import {ApiResponse, BasicUserInfo} from '../core/types';

/**
 * 用户名密码登录请求参数
 */
export interface LoginRequest {
  /** 用户名 */
  username: string;
  /** 密码 */
  password: string;
}

/**
 * 登录响应数据
 */
export interface LoginResponseData {
  /** 用户ID */
  id: number;
  /** 用户名 */
  username: string;
  /** 昵称 */
  nickname: string;
  /** 是否管理员 */
  is_admin: boolean;
}

/**
 * 登录响应
 */
export type LoginResponse = ApiResponse<LoginResponseData>;

/**
 * OAuth登录URL响应
 */
export type GetLoginURLResponse = ApiResponse<string>;

/**
 * OAuth回调请求参数
 */
export interface CallbackRequest {
  /** OAuth状态码，用于验证请求合法性 */
  state: string;
  /** 授权码，用于获取访问令牌 */
  code: string;
}

/**
 * OAuth回调响应
 */
export type CallbackResponse = ApiResponse<null>;

/**
 * 用户信息响应
 */
export type UserInfoResponse = ApiResponse<BasicUserInfo>;
