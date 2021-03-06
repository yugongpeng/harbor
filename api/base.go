/*
   Copyright (c) 2016 VMware, Inc. All Rights Reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/vmware/harbor/auth"
	"github.com/vmware/harbor/dao"
	"github.com/vmware/harbor/models"
	"github.com/vmware/harbor/utils"

	"github.com/astaxie/beego"
	redis "github.com/garyburd/redigo/redis"
)

// BaseAPI wraps common methods for controllers to host API
type BaseAPI struct {
	beego.Controller
}

// Render returns nil as it won't render template
func (b *BaseAPI) Render() error {
	return nil
}

// RenderError provides shortcut to render http error
func (b *BaseAPI) RenderError(code int, text string) {
	http.Error(b.Ctx.ResponseWriter, text, code)
}

// DecodeJSONReq decodes a json request
func (b *BaseAPI) DecodeJSONReq(v interface{}) {
	err := json.Unmarshal(b.Ctx.Input.CopyBody(1<<32), v)
	if err != nil {
		beego.Error("Error while decoding the json request:", err)
		b.CustomAbort(http.StatusBadRequest, "Invalid json request")
	}
}

// ValidateUser checks if the request triggered by a valid user
func (b *BaseAPI) ValidateUser() int {

	// validate user from basic auth
	username, password, ok := b.Ctx.Request.BasicAuth()
	if ok {
		log.Printf("Requst with Basic Authentication header, username: %s", username)
		user, err := auth.Login(models.AuthModel{username, password})
		if err != nil {
			log.Printf("Error while trying to login, username: %s, error: %v", username, err)
			user = nil
		}
		if user != nil {
			return user.UserID
		}
	}

	// validate user from token
	token := b.Ctx.Request.Header.Get("Authorization")

	if len(token) == 0 {
		token = b.GetString("authorization")
	}

	if len(token) > 0 {
		conn := utils.OpenRedisPool()
		defer conn.Close()
		uid_, err := redis.String(conn.Do("HGET", "s:"+token, "user_id"))

		if err != nil {
			beego.Warning("No user id in session, canceling request")
			b.CustomAbort(http.StatusUnauthorized, "")
		}
		uid, _ := strconv.Atoi(uid_)
		return uid
	}

	// validate user from session
	sessionUserID := b.GetSession("userId")
	if sessionUserID == nil {
		beego.Warning("No user id in session, canceling request")
		b.CustomAbort(http.StatusUnauthorized, "")
	}
	userID := sessionUserID.(int)
	u, err := dao.GetUser(models.User{UserID: userID})
	if err != nil {
		beego.Error("Error occurred in GetUser:", err)
		b.CustomAbort(http.StatusInternalServerError, "Internal error.")
	}
	if u == nil {
		beego.Warning("User was deleted already, user id: ", userID, " canceling request.")
		b.CustomAbort(http.StatusUnauthorized, "")
	}
	return userID
}
