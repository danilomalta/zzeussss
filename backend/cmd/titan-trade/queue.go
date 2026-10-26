package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/traderelay"
)

const queueHelp = `TitanSystem trade queue — fila privada de mensagens previamente autorizadas
Comandos:
  queue init --queue /PASTA_PRIVADA_NOVA
  queue add --queue PASTA --job ARQUIVO --source-token ARQUIVO --target-token ARQUIVO
  queue backup --queue PASTA --key ARQUIVO_PRIVADO --out ARQUIVO_NOVO
  queue verify-backup --archive ARQUIVO --key ARQUIVO_PRIVADO
  queue restore --archive ARQUIVO --key ARQUIVO_PRIVADO --queue PASTA_NOVA
  queue status --queue PASTA
  queue run --queue PASTA --limit 10
  queue watch --queue PASTA --limit 10 --interval 10s
  queue verify --queue PASTA --id ITEM
  queue pause --queue PASTA --id ITEM
  queue resume --queue PASTA --id ITEM --source-token ARQUIVO --target-token ARQUIVO
  queue retry --queue PASTA --id ITEM
  queue recover-session --queue PASTA --id ITEM --target-token ARQUIVO
A fila guarda somente configuração explícita, hashes e estado, nunca tokens ou documentos.
Pasta0700; arquivos0600. Troca/expiração de sessão pausa o item, sem apagá-lo.
Mensagem incerta: consulta o recibo;404 exige retry explícito. Não reenvia automaticamente.
watch só entrega mensagens já autorizadas na fila; não aprova compra, não decide atendimento e não cria credenciais.
status/exit0 do lote não significam aceite comercial: consulte confirmed/uncertain/paused/needs_retry.
recover-session reautoriza SOMENTE consulta de recibo com prova original; não exige origem online nem permite POST.
Restauração fica pausada e permanentemente limitada à consulta dos recibos originais.
Backup da fila não inclui banco, tokens nem chave; preserve também o backup oficial do banco.
Não instala serviço automaticamente nem modifica banco ou configuração de rede.`

func runQueue(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		_, e := fmt.Fprintln(out, queueHelp)
		return e
	}
	if ctx == nil || len(args) == 0 {
		return traderelay.ErrInvalid
	}
	command := args[0]
	switch command {
	case "recover-session", "backup", "verify-backup", "restore", "init", "add", "status", "run", "watch", "pause", "resume", "retry", "verify":
	default:
		return traderelay.ErrInvalid
	}
	fs := flag.NewFlagSet("titan-trade queue "+command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("queue", "", "")
	var job, st, tt, id *string
	var keyFile, archive, output *string
	if command == "backup" || command == "verify-backup" || command == "restore" {
		keyFile = fs.String("key", "", "")
	}
	if command == "backup" {
		output = fs.String("out", "", "")
	}
	if command == "verify-backup" || command == "restore" {
		archive = fs.String("archive", "", "")
	}
	var limit *int
	var interval *time.Duration
	if command == "add" {
		job = fs.String("job", "", "")
	}
	if command == "add" || command == "resume" {
		st = fs.String("source-token", "", "")
	}
	if command == "add" || command == "resume" || command == "recover-session" {
		tt = fs.String("target-token", "", "")
	}
	if command == "pause" || command == "resume" || command == "retry" || command == "verify" || command == "recover-session" {
		id = fs.String("id", "", "")
	}
	if command == "run" || command == "watch" {
		limit = fs.Int("limit", 10, "")
	}
	if command == "watch" {
		interval = fs.Duration("interval", 10*time.Second, "")
	}
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 || (*dir == "" && command != "verify-backup") {
		return traderelay.ErrInvalid
	}
	encoder := json.NewEncoder(out)
	if keyFile != nil {
		if *keyFile == "" || (archive != nil && *archive == "") || (output != nil && *output == "") || (command == "verify-backup" && *dir != "") {
			return traderelay.ErrInvalid
		}
		key, e := backup.ReadKey(*keyFile)
		if e != nil {
			return e
		}
		defer func() {
			for n := range key {
				key[n] = 0
			}
		}()
		var info traderelay.QueueBackupInfo
		switch command {
		case "backup":
			info, e = traderelay.QueueBackup(*dir, key, *output)
		case "verify-backup":
			info, e = traderelay.QueueVerifyBackup(*archive, key)
		case "restore":
			info, e = traderelay.QueueRestore(*archive, key, *dir)
		}
		if e != nil {
			return e
		}
		return encoder.Encode(info)
	}
	if command == "watch" {
		if *interval < time.Second || *interval > 5*time.Minute || *limit < 1 || *limit > 100 {
			return traderelay.ErrInvalid
		}
		if _, e := traderelay.QueueRead(*dir); e != nil {
			return e
		}
		if e := encoder.Encode(map[string]bool{"watching": true}); e != nil {
			return e
		}
		for {
			if ctx.Err() != nil {
				return nil
			}
			pass, cancel := context.WithTimeout(ctx, 2*time.Minute)
			batch, e := traderelay.QueueRun(pass, *dir, *limit, time.Now().UTC())
			cancel()
			if ctx.Err() != nil {
				return nil
			}
			if e != nil && !errors.Is(e, traderelay.ErrLocked) && !errors.Is(e, context.DeadlineExceeded) {
				return e
			}
			if batch.Processed > 0 {
				if e = encoder.Encode(batch); e != nil {
					return e
				}
			}
			timer := time.NewTimer(*interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	switch command {
	case "init":
		if e := traderelay.QueueInit(*dir); e != nil {
			return e
		}
		return encoder.Encode(map[string]bool{"initialized": true})
	case "status":
		view, e := traderelay.QueueRead(*dir)
		if e != nil {
			return e
		}
		return encoder.Encode(view)
	case "add":
		result, e := traderelay.QueueAdd(bounded, *dir, *job, *st, *tt)
		if e != nil {
			return e
		}
		return encoder.Encode(result)
	case "recover-session":
		result, e := traderelay.QueueRecoverSession(bounded, *dir, *id, *tt)
		if e != nil {
			return e
		}
		return encoder.Encode(result)
	case "resume":
		result, e := traderelay.QueueResume(bounded, *dir, *id, *st, *tt)
		if e != nil {
			return e
		}
		return encoder.Encode(result)
	case "pause":
		if e := traderelay.QueuePause(*dir, *id); e != nil {
			return e
		}
		return encoder.Encode(map[string]bool{"paused": true})
	case "run":
		batch, e := traderelay.QueueRun(bounded, *dir, *limit, time.Now().UTC())
		if e != nil {
			return e
		}
		return encoder.Encode(batch)
	case "verify":
		batch, e := traderelay.QueueVerify(bounded, *dir, *id, time.Now().UTC())
		if e != nil {
			return e
		}
		return encoder.Encode(batch)
	case "retry":
		batch, e := traderelay.QueueRetry(bounded, *dir, *id, time.Now().UTC())
		if e != nil {
			return e
		}
		return encoder.Encode(batch)
	}
	return traderelay.ErrInvalid
}
