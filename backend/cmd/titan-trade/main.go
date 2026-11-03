package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
	"titansystem-backend/internal/traderelay"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if e := run(ctx, os.Args[1:], os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, "Titan trade:", e)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "queue" {
		return runQueue(ctx, args[1:], out)
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		_, e := fmt.Fprintln(out, `TitanSystem trade — transporte explícito de mensagens assinadas (Linux)
Uso: titan-trade --job JOB_PRIVADO --state /PASTA_PRIVADA/TENTATIVA --source-token TOKEN_ORIGEM --target-token TOKEN_DESTINO --mode send|recover|retry
send transmite uma mensagem já exportada, sem aprovar pedido ou decidir atendimento.
recover usa somente o token do destino, mesmo com a origem offline, e nunca faz POST. retry é reenvio explícito com os mesmos IDs após consulta404.
Arquivos de job/token:0600; pasta de state:0700. HTTPS obrigatório fora de loopback; certificado validado pelo sistema.
Nenhum token, envelope ou documento comercial é escrito na saída ou no state. Preserve job e state na recuperação.
Modo avulso não agenda envios. Fila operacional: titan-trade queue help.
Não instala servidor/relay público, não cria credenciais e não confirma fornecedor por conta própria.`)
		return e
	}
	fs := flag.NewFlagSet("titan-trade", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	job := fs.String("job", "", "")
	state := fs.String("state", "", "")
	st := fs.String("source-token", "", "")
	tt := fs.String("target-token", "", "")
	mode := fs.String("mode", "", "")
	if fs.Parse(args) != nil || fs.NArg() != 0 {
		return traderelay.ErrInvalid
	}
	if *mode != "send" && *mode != "recover" && *mode != "retry" {
		return traderelay.ErrInvalid
	}
	j, e := traderelay.ReadJob(*job)
	if e != nil {
		return e
	}
	source := ""
	if *mode != "recover" {
		source, e = traderelay.ReadToken(*st)
		if e != nil {
			return e
		}
	}
	target, e := traderelay.ReadToken(*tt)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	r, e := traderelay.Run(ctx, j, source, target, *state, *mode)
	if e != nil {
		return e
	}
	if r.Status != "confirmed" {
		return errors.New("delivery not confirmed")
	}
	return json.NewEncoder(out).Encode(r)
}
