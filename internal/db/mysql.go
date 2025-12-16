/*
 * MIT License
 *
 * Copyright (c) 2025 linux.do
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package db

import (
	"context"
	"log"

	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

// init 函数已废弃，数据库初始化统一由 database.go 中的 InitDatabase() 处理
// 保留此文件仅用于向后兼容，不再自动初始化 MySQL 连接
func init() {
	// 不再在 init 中自动初始化数据库
	// 数据库初始化现在由 InitDatabase() 函数统一处理
	// 该函数会根据配置文件中的 database.type 选择正确的数据库驱动
	log.Println("[MySQL] init() 已废弃，数据库初始化由 InitDatabase() 统一处理")
}

// DB 函数已废弃，请使用 GetDB(ctx) 代替
// 保留此函数仅用于向后兼容
func DB(ctx context.Context) *gorm.DB {
	// 使用 database.go 中的统一数据库实例
	return GetDB(ctx)
}
