package handlers

import "net/http"

func NewVerifyEmailStartHandler(h *EmailFlowsHandler) http.HandlerFunc   { return h.verifyEmailStart }
func NewVerifyEmailConfirmHandler(h *EmailFlowsHandler) http.HandlerFunc { return h.verifyEmailConfirm }
func NewForgotHandler(h *EmailFlowsHandler) http.HandlerFunc             { return h.forgot }
func NewResetHandler(h *EmailFlowsHandler) http.HandlerFunc              { return h.reset }
