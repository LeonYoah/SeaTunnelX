/**
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {BaseService} from '../core/base.service';

/**
 * 用户信息
 */
export interface UserInfo {
  id: number;
  username: string;
  nickname: string;
  avatar_url?: string;
  is_active: boolean;
  is_admin: boolean;
  last_login_at: string;
  created_at: string;
}

/**
 * 用户列表请求参数
 */
export interface ListUsersParams {
  current: number;
  size: number;
  username?: string;
  is_active?: boolean;
  is_admin?: boolean;
}

/**
 * 用户列表响应
 */
export interface ListUsersResponse {
  total: number;
  users: UserInfo[];
}

/**
 * 创建用户请求
 */
export interface CreateUserRequest {
  username: string;
  password: string;
  nickname?: string;
  is_admin?: boolean;
}

/**
 * 更新用户请求
 */
export interface UpdateUserRequest {
  nickname?: string;
  password?: string;
  is_active?: boolean;
  is_admin?: boolean;
}

/**
 * 管理员用户管理服务
 */
export class AdminUserService extends BaseService {
  protected static readonly basePath = '/admin/users';

  /**
   * 获取用户列表
   */
  static async listUsers(params: ListUsersParams): Promise<ListUsersResponse> {
    const queryParams = new URLSearchParams();
    queryParams.append('current', params.current.toString());
    queryParams.append('size', params.size.toString());
    if (params.username) {
      queryParams.append('username', params.username);
    }
    if (params.is_active !== undefined) {
      queryParams.append('is_active', params.is_active.toString());
    }
    if (params.is_admin !== undefined) {
      queryParams.append('is_admin', params.is_admin.toString());
    }

    return this.get<ListUsersResponse>(`?${queryParams.toString()}`);
  }

  /**
   * 获取单个用户
   */
  static async getUser(id: number): Promise<UserInfo> {
    return this.get<UserInfo>(`/${id}`);
  }

  /**
   * 创建用户
   */
  static async createUser(data: CreateUserRequest): Promise<UserInfo> {
    return this.post<UserInfo>('', data);
  }

  /**
   * 更新用户
   */
  static async updateUser(
    id: number,
    data: UpdateUserRequest,
  ): Promise<UserInfo> {
    return this.put<UserInfo>(`/${id}`, data);
  }

  /**
   * 删除用户
   */
  static async deleteUser(id: number): Promise<void> {
    return this.delete(`/${id}`);
  }
}
