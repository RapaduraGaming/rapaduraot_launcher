# RapaduraOT Launcher

Launcher oficial do [RapaduraOT](https://ot.rapadura.org), responsavel por instalar, atualizar e iniciar o cliente do jogo automaticamente.

## O que faz

- **Instalacao automatica** - Na primeira execucao, baixa e instala o cliente na pasta `%LOCALAPPDATA%\RapaduraOT`
- **Atualizacao automatica** - Verifica a versao mais recente via API e atualiza o cliente antes de iniciar
- **Auto-atualizacao do launcher** - O proprio launcher se atualiza quando uma versao nova esta disponivel
- **Verificacao de integridade** - Valida o checksum SHA256 dos arquivos baixados
- **Atalhos** - Cria atalhos na area de trabalho e no menu iniciar automaticamente
- **System tray** - Minimiza para a bandeja do sistema enquanto o jogo esta aberto
- **Instancia unica** - Impede que multiplas copias do launcher rodem ao mesmo tempo

## Download

Baixe a versao mais recente na pagina de [Releases](../../releases). Existem duas opcoes:

### Instalador (recomendado)

Baixe o **`RapaduraOT-Setup.exe`** e execute. O instalador cuida de tudo:

- Instala o launcher em `%LOCALAPPDATA%\RapaduraOT`
- Cria atalho na area de trabalho
- Cria entrada no menu iniciar
- Registra no "Adicionar ou remover programas" do Windows para desinstalacao

### ZIP portatil

Baixe o **`RapaduraOTLauncher.zip`**, extraia e execute o `RapaduraOTLauncher.exe` diretamente. Na primeira execucao o launcher baixa o cliente automaticamente. Os atalhos nao sao criados nesse modo.

## Compilando

### Requisitos

- [Go 1.21+](https://go.dev/dl/)
- Windows (o launcher usa APIs nativas do Windows)

### Build

```bash
cd launcher
go mod tidy
go build -ldflags="-H windowsgui -s -w" -o RapaduraOTLauncher.exe .
```

Ou use o script pronto:

```bash
build.bat
```

### Gerando o instalador

Requer [NSIS](https://nsis.sourceforge.io/) instalado:

```bash
cd installer
build_launcher_installer.bat
```

Isso gera o `RapaduraOT-Setup.exe`.

## Estrutura

```
main.go         # Ponto de entrada, janela principal e loop de eventos
ui.go           # Renderizacao da interface com tema escuro
updater.go      # Download e instalacao do cliente via API
selfupdate.go   # Auto-atualizacao do launcher
install.go      # Diretorio de instalacao e criacao de atalhos
tray.go         # Icone na bandeja do sistema
winclient.go    # Gerenciamento do processo do cliente
assets/         # Logo e icone embutidos no executavel
winres/         # Recursos do Windows (icone, manifesto)
installer/      # Scripts NSIS para gerar o instalador
```

## API

O launcher consulta a API em `https://api.rapadura.org/api/v1/client/version` para obter:

| Campo | Descricao |
|-------|-----------|
| `version` | Versao atual do cliente |
| `downloadUrl` | URL do ZIP para download |
| `checksum` | Hash SHA256 para verificacao |
| `launcherVersion` | Versao mais recente do launcher |
| `launcherUrl` | URL do novo launcher (para auto-atualizacao) |

## Licenca

Uso interno. Todos os direitos reservados.
