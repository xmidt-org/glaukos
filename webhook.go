/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"time"

	"github.com/xmidt-org/bascule/acquire"
	webhook "github.com/xmidt-org/wrp-listener"
	"github.com/xmidt-org/wrp-listener/webhookClient"
)

type WebhookConfig struct {
	RegistrationInterval time.Duration
	Timeout              time.Duration
	RegistrationURL      string
	HostToRegister       string
	Request              webhook.W
	JWT                  acquire.RemoteBearerTokenAcquirerOptions
	Basic                string
}

// determineTokenAcquirer always returns a valid TokenAcquirer
func determineTokenAcquirer(config WebhookConfig) (webhookClient.Acquirer, error) {
	defaultAcquirer := &acquire.DefaultAcquirer{}
	if config.JWT.AuthURL != "" && config.JWT.Buffer != 0 && config.JWT.Timeout != 0 {
		return acquire.NewRemoteBearerTokenAcquirer(config.JWT)
	}

	if config.Basic != "" {
		return acquire.NewFixedAuthAcquirer(config.Basic)
	}

	return defaultAcquirer, nil
}
