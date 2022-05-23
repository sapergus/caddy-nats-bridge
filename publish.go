package caddynats

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Publish{})
}

type Publish struct {
	//TODO use repl on subject to get placeholder support
	Subject   string `json:"subject,omitempty"`
	WithReply bool   `json:"with_reply,omitempty"`

	logger *zap.Logger
	app    *App
}

func (Publish) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.nats_publish",
		New: func() caddy.Module { return new(Publish) },
	}
}

func (p *Publish) Provision(ctx caddy.Context) error {
	p.logger = ctx.Logger(p)

	natsAppIface, err := ctx.App("nats")
	if err != nil {
		return fmt.Errorf("getting nats app: %v. Make sure nats is configured in global options", err)
	}

	p.app = natsAppIface.(*App)

	return nil
}

func (p *Publish) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if !d.NextArg() {
			return d.Errf("Not enough arguments", d.Val())
		}
		p.Subject = d.Val()
		if d.NextArg() {
			return d.Errf("Wrong argument count or unexpected line ending after '%s'", d.Val())
		}
	}

	return nil
}

func (p Publish) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	//TODO: Check max msg size
	//TODO: configurable bodies
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if p.WithReply {
		return p.natsRequestResponse(data, w)
	}

	// Otherwise. just publish like normal
	err = p.app.conn.Publish(p.Subject, data)
	if err != nil {
		return err
	}

	return next.ServeHTTP(w, r)
}

func (p Publish) natsRequestResponse(reqBody []byte, w http.ResponseWriter) error {
	//TODO: Configurable timeout
	m, err := p.app.conn.Request(p.Subject, reqBody, time.Second*10)
	if err != nil {
		return err
	}

	if err == nats.ErrNoResponders {
		w.WriteHeader(http.StatusNotFound)
		return err
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	_, err = w.Write(m.Data)

	return err
}

var (
	_ caddyhttp.MiddlewareHandler = (*Publish)(nil)
	_ caddy.Provisioner           = (*Publish)(nil)
	_ caddyfile.Unmarshaler       = (*Publish)(nil)
)
