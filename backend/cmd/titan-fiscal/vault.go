package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"titansystem-backend/internal/fiscalproduction"
)

func privateTerminal() (io.WriteCloser, error) {
	name := "/dev/tty"
	if runtime.GOOS == "windows" {
		name = "CONOUT$"
	}
	f, err := os.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return nil, fiscalproduction.ErrUnavailable
	}
	return f, nil
}
func vaultCommand(args []string, out io.Writer, terminal func() (io.WriteCloser, error)) error {
	fs := flag.NewFlagSet("vault", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	vault := fs.String("vault", "", "customer vault")
	key := fs.String("key-file", "", "customer private key file")
	dest := fs.String("out", "", "new customer key backup/file")
	locale := fs.String("lang", "pt-BR", "language")
	confirmed := fs.Bool("backup-confirmed", false, "customer confirms old key backup")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 || *key == "" {
		return errors.New("invalid customer vault arguments")
	}
	words := map[string][2]string{
		"pt-BR": {"Chave privada exibida uma vez neste terminal. Perder a chave perde os segredos. O provedor não pode recuperar. Faça seu backup privado.", "Operação local concluída. Nenhuma chave foi enviada ao provedor."},
		"en":    {"Private key shown once in this terminal. Losing the key loses the secrets. The vendor cannot recover it. Make your private backup.", "Local operation completed. No key was sent to the vendor."},
		"es":    {"Clave privada mostrada una vez en esta terminal. Perderla implica perder los secretos. El proveedor no puede recuperarla. Haga su copia privada.", "Operación local completada. Ninguna clave fue enviada al proveedor."},
		"fr":    {"Clé privée affichée une fois dans ce terminal. La perdre rend les secrets inaccessibles. Le fournisseur ne peut pas la récupérer. Faites votre sauvegarde privée.", "Opération locale terminée. Aucune clé envoyée au fournisseur."},
		"zh-CN": {"私钥仅在此终端显示一次。丢失私钥将无法读取凭据，供应商无法恢复。请自行制作私密备份。", "本地操作已完成，未向供应商发送任何密钥。"},
	}
	w, ok := words[*locale]
	if !ok {
		return errors.New("unsupported language")
	}
	if args[0] == "vault-export-key" {
		if *dest == "" || *vault != "" {
			return fiscalproduction.ErrUnavailable
		}
		if err := fiscalproduction.ExportCustomerMasterKey(*key, *dest); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, w[1])
		return err
	}
	if *vault == "" || (args[0] != "vault-init" && args[0] != "vault-rotate") || (args[0] == "vault-init" && *dest != "") || (args[0] == "vault-rotate" && (*dest == "" || !*confirmed)) {
		return fiscalproduction.ErrUnavailable
	}
	tty, err := terminal()
	if err != nil {
		return fiscalproduction.ErrUnavailable
	}
	defer tty.Close()
	if _, err = fmt.Fprintln(tty, w[0]); err != nil {
		return fiscalproduction.ErrUnavailable
	}
	if args[0] == "vault-init" {
		err = fiscalproduction.CreateCustomerFiscalVault(*vault, *key, tty)
	} else {
		err = fiscalproduction.RotateCustomerMasterKey(*vault, *key, *dest, tty)
	}
	if err != nil {
		return fiscalproduction.ErrUnavailable
	}
	_, err = fmt.Fprintln(out, w[1])
	return err
}
