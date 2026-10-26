# Correção de abertura no Ubuntu com AppArmor

25/10/2026. Instalação Linux atual, Electron44.9.2. O terminal do agente conseguia abrir; a sessão GNOME abortava antes de mostrar a janela. O journal mostrou FATAL setuid_sandbox_host.cc:166. Não tratar esse erro como demora de API ou resolver com --no-sandbox.

## Configuração realmente instalada

`/etc/apparmor.d/titansystem-desktop-userns`, proprietário root:root, modo0644:

```apparmor
# TitanSystem installed Electron: allow Chromium's user-namespace sandbox.
# No global sysctl changes; no setuid helper and no --no-sandbox.
abi <abi/4.0>,
include <tunables/global>

profile titansystem_desktop "/home/danillo-malta/.local/opt/titansystem/runtime/electron-44.9.2-linux-x64/electron" flags=(unconfined) {
  userns,
}
```

Formato baseado nos perfis Chrome e VSCode deste Ubuntu. AppArmor atribui um nome ao executável sem confinamento adicional e permite criar os user namespaces necessários ao sandbox Chromium. O caminho é exato; não estender a todos os executáveis do usuário. A permissão é por caminho, não assinatura/verificação criptográfica do binário. Não usar isso como afirmação de confinamento AppArmor integral do ERP.

A regra foi validada com apparmor_parser --skip-kernel-load --skip-cache antes da aplicação. Administração feita por pkexec com autenticação do Ubuntu; nenhuma senha recebida pelo agente. O aplicador recusava arquivo preexistente e remove sua própria criação se a carga falhar. sysctl global continua restritivo; chrome-sandbox755 do usuário não foi convertido em setuid. Nenhuma dependência instalada ou backend reiniciado.

## Entrada correta

Novo ID `com.titansystem.ERP.desktop`, nome TitanSystem ERP, Exec `~/.local/bin/titansystem-desktop`, DBusActivatable=false, StartupWMClass=titansystem-desktop. O lançador abre interface e runtime compilados já instalados, sem Vite/build/extração. Entrada anterior ocultada, com backup; atalhos Downloads/área de trabalho atualizados. O kernel não foi configurado para iniciar o backend automaticamente.

Teste fora do ambiente do Codex: serviço da própria sessão abriu em0,705s; GTK registrou janela em0,684s. A instância encontrada no escopo app-gnome-com.titansystem.ERP permaneceu aberta. Verificação da sessão confirmou três filhos com seccomp2 e NoNewPrivs1; os tipos internos individuais não foram identificados. Não houve login com conta comercial.

## Manutenção e reversão

Atualizar o runtime em outro caminho exige revisar e carregar a regra específica para esse executável; não desligar a proteção global. Antes de repetir a instalação, comparar o arquivo existente e preservar qualquer edição administrativa. Reversão requer administrador descarregar somente esse perfil e remover somente o arquivo gerenciado depois de conferir seu hash. Os atalhos anteriores estão preservados em .gnome-fix-*.bak; restaurá-los não corrige a ausência de permissão de sandbox. O trabalho comercial e os bancos não dependem dessas cópias de atalhos.

Linux desta máquina validado; isso não prova Windows, distribuição genérica para outro Ubuntu, todas as telas ou autenticação. As medições anteriores feitas apenas no terminal do agente não eram prova suficiente de abertura pelo usuário.
