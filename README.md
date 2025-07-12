# 🏆 Rinha de Backend 2025

> **Arquitetura de Microsserviços com gRPC, mTLS e Autenticação Signet-go - 100% Conforme Desafio**

Uma solução robusta para processamento de pagamentos com arquitetura distribuída, segurança avançada e persistência confiável, **totalmente aderente aos requisitos da Rinha de Backend 2025**.

## 🏗️ **Arquitetura de Microsserviços**

```
┌─────────────────┐    HTTP/REST    ┌─────────────────┐
│   Cliente HTTP  │ ──────────────► │   API Gateway   │
│   (Porta 9999)  │                 │   (Container)   │
└─────────────────┘                 └─────────────────┘
                                              │
                                              │ gRPC + mTLS + Signet
                                              ▼
┌─────────────────┐                 ┌─────────────────┐
│   Processador   │ ◄────────────── │ Payment         │
│   Externo       │                 │ Orchestrator    │
│   (Rede Externa)│                 │   (Container)   │
└─────────────────┘                 └─────────────────┘
                                              │
                                              │ gRPC + mTLS + Signet
                                              ▼
                                    ┌─────────────────┐
                                    │ Summary Service │
                                    │   (Container)   │
                                    └─────────────────┘
                                              │
                                              ▼
                                    ┌─────────────────┐
                                    │   BBolt DB      │
                                    │   (Persistência)│
                                    └─────────────────┘
```

### **Estrutura de Containers**
- **API Gateway**: Ponto de entrada HTTP (porta 9999)
- **Payment Orchestrator**: Orquestra pagamentos internamente
- **Summary Service**: Gerencia resumos e consultas
- **Rede Compartilhada**: Comunicação segura entre microsserviços
- **Recursos Limitados**: CPU e memória controlados por container

## 🔐 **Segurança**

- **mTLS (Mutual TLS)**: Autenticação bidirecional com certificados X.509
- **Signet-go**: Autenticação baseada em tokens JWT com chaves Ed25519
- **Rate Limiting**: Proteção contra abuso de API
- **Health Checks**: Monitoramento de saúde dos serviços
- **Fallback**: Redundância entre processadores de pagamento

## ✅ **Conformidade com Rinha de Backend 2025**

### **Endpoints Obrigatórios - 100% Implementados**

#### ✅ POST /payments
- **Formato de entrada**: `{ "correlationId": "uuid", "amount": decimal }`
- **Validação**: UUID obrigatório, amount > 0
- **Resposta**: HTTP 2XX (qualquer)
- **Porta**: 9999

#### ✅ GET /payments-summary
- **Parâmetros**: `from` e `to` opcionais (ISO UTC)
- **Resposta**: 
  ```json
  {
    "default": { "totalRequests": int, "totalAmount": decimal },
    "fallback": { "totalRequests": int, "totalAmount": decimal }
  }
  ```

### **Integração com Payment Processors - 100% Implementada**

#### ✅ Payload Correto
- **Enviado aos processadores**: `{ "correlationId": "uuid", "amount": decimal, "requestedAt": "ISO UTC" }`
- **Health Check**: `/payments/service-health` com rate limit 1 req/5s
- **URLs**: `http://payment-processor-default:8080` e `http://payment-processor-fallback:8080`

### **Docker Compose - 100% Conforme**

#### ✅ Requisitos Atendidos
- **Porta 9999**: API Gateway exposto na porta correta
- **2+ Instâncias**: Pelo menos 2 instâncias do serviço HTTP
- **Limites de Recursos**: 1.5 CPU e 350MB RAM total
- **Rede Externa**: Conecta à rede `payment-processor`
- **Plataforma**: Build para `linux/amd64`

### **Validações e Testes - 100% Implementados**

#### ✅ Validações de Entrada
- **correlationId**: UUID obrigatório e válido
- **amount**: Decimal obrigatório > 0
- **Métodos HTTP**: Validação de métodos permitidos

#### ✅ Testes Automatizados
- **Script de Conformidade**: `scripts/test-rinha-compliance.ps1`
- **Cobertura**: Todos os endpoints e validações
- **Formato**: Validação de estrutura de resposta

## 📊 **Status do Épico**

### ✅ **História 1: A Segurança Interna** - CONCLUÍDA
- [x] Implementação de mTLS entre serviços
- [x] Autenticação signet-go com chaves Ed25519
- [x] Rate limiting e health checks
- [x] Fallback entre processadores

### ✅ **História 2: A Lógica Principal** - CONCLUÍDA
- [x] Payment Orchestrator com lógica de negócio
- [x] Integração com processadores externos
- [x] Tratamento de erros e retry logic
- [x] Testes automatizados com mocks

### ✅ **História 3: A Fachada Pública** - CONCLUÍDA
- [x] API Gateway com endpoints HTTP/REST
- [x] Ponte para serviços internos via gRPC
- [x] Documentação de API e exemplos
- [x] Testes de integração

### ✅ **História 4: O Resumo Inteligente** - CONCLUÍDA
- [x] Summary Service para consultas de resumo
- [x] Agregação de dados de pagamentos
- [x] Endpoints para consulta de histórico
- [x] Integração com API Gateway

### ✅ **História 5: A Persistência** - CONCLUÍDA
- [x] Camada de persistência com BBolt
- [x] Buckets automáticos
- [x] Operações CRUD completas
- [x] Recuperação de dados após reinicialização
- [x] Estatísticas e consultas otimizadas

### ✅ **Conformidade Rinha 2025** - CONCLUÍDA
- [x] Endpoints no formato exato do desafio
- [x] Integração correta com Payment Processors
- [x] Docker Compose conforme especificações
- [x] Validações e testes automatizados
- [x] Documentação e info.json

## 🚀 **Início Rápido**

### Pré-requisitos
- Go 1.21+
- OpenSSL
- buf (será instalado automaticamente)

### Instalação e Execução

```bash
# 1. Clone o repositório
git clone <repository-url>
cd rinha-de-backend-2025

# 2. Prepare o ambiente
./scripts/dev.sh

# 3. Teste a conformidade com a Rinha
powershell -ExecutionPolicy Bypass -File scripts/test-rinha-compliance.ps1
```

### Execução via Docker Compose (Recomendado)

```bash
# 1. Inicia todos os microsserviços
docker-compose up -d

# 2. Verifica status dos containers
docker-compose ps

# 3. Acessa logs de um serviço específico
docker-compose logs api-gateway-1
docker-compose logs payment-orchestrator
docker-compose logs summary-service
```

### Execução Manual (Desenvolvimento)

```bash
# Terminal 1: Processadores Mock (externos)
./mock-processor.exe
./mock-processor.exe fallback

# Terminal 2: Microsserviços Internos
./payment-orchestrator.exe
./summary-service.exe

# Terminal 3: API Gateway (porta 9999)
./api-gateway.exe
```

## 📡 **API Endpoints (Conforme Rinha 2025)**

### POST /payments
Cria um novo pagamento.

```bash
curl -X POST http://localhost:9999/payments \
  -H "Content-Type: application/json" \
  -d '{
    "correlationId": "4a7901b8-7d26-4d9d-aa19-4dc1c7cf60b3",
    "amount": 100.50
  }'
```

**Resposta:**
```json
{
  "id": "pay_abc123",
  "status": "completed",
  "message": "Pagamento processado com sucesso"
}
```

### GET /payments-summary
Consulta resumo de pagamentos por processador.

```bash
curl "http://localhost:9999/payments-summary"
```

**Resposta:**
```json
{
  "default": {
    "totalRequests": 150,
    "totalAmount": 15000.75
  },
  "fallback": {
    "totalRequests": 25,
    "totalAmount": 2500.50
  }
}
```

## 🗄️ **Persistência de Dados**

### Banco de Dados BBolt
- **Localização**: `./data/payments.db`
- **Buckets**: Automáticos na inicialização
- **Serialização**: Gob encoding para performance máxima

### Estrutura dos Dados
```go
type Payment struct {
    ID            string    `json:"id"`
    CustomerID    string    `json:"customer_id"`
    Amount        float64   `json:"amount"`
    Description   string    `json:"description"`
    Status        string    `json:"status"`
    ProcessorUsed string    `json:"processor_used"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

## 🧪 **Testes**

### 🏆 Teste Oficial dos Avaliadores (Recomendado)
```bash
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts/teste-oficial-avaliadores.ps1

# Linux/Mac (Bash)
./scripts/teste-oficial-avaliadores.sh
```

**Requisitos:**
- k6 instalado (https://grafana.com/docs/k6/latest/set-up/install-k6/)
- Docker e Docker Compose
- Porta 9999 disponível

**O que testa:**
- ✅ Endpoints exatos do desafio
- ✅ Integração com Payment Processors oficiais
- ✅ Performance e carga
- ✅ Consistência de dados
- ✅ Cenários de falha e fallback
- ✅ Dashboard em tempo real (http://localhost:5665)

### Teste de Conformidade Rinha 2025
```bash
powershell -ExecutionPolicy Bypass -File scripts/test-rinha-compliance.ps1
```

**Cobertura:**
- ✅ Formato correto dos endpoints
- ✅ Validação de UUID e campos obrigatórios
- ✅ Estrutura de resposta do summary
- ✅ Parâmetros opcionais
- ✅ Porta 9999

### Prova de Fogo (Teste Completo)
```bash
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts/prova-de-fogo.ps1

# Linux/Mac (Bash)
./scripts/prova-de-fogo.sh
```

**Cobertura:**
- ✅ Validações de entrada
- ✅ Performance com múltiplas requisições
- ✅ Persistência após reinicialização
- ✅ Logs e recursos dos containers

## 🔧 **Desenvolvimento**

### Estrutura do Projeto
```
rinha-de-backend-2025/
├── cmd/                    # Executáveis principais
│   ├── api-gateway/       # API Gateway (porta 9999)
│   ├── payment-orchestrator/ # Orquestrador de pagamentos
│   └── summary-service/   # Serviço de resumo
├── internal/              # Código interno
│   ├── database/          # Camada de persistência
│   ├── gen/proto/         # Stubs protobuf gerados
│   ├── keys/              # Gerenciamento de chaves
│   └── resolver/          # Resolução de chaves
├── proto/                 # Definições protobuf
├── certs/                 # Certificados mTLS
├── config/                # Configurações
├── examples/              # Exemplos e testes
├── scripts/               # Scripts de automação
├── data/                  # Banco de dados BBolt
├── docker-compose.yml     # Conforme Rinha 2025
└── info.json              # Tecnologias utilizadas
```

### Comandos Úteis

```bash
# Gerar stubs protobuf
buf generate

# Atualizar dependências
go mod tidy

# Compilar todos os serviços
./scripts/dev.sh

# Testar conformidade Rinha 2025
powershell -ExecutionPolicy Bypass -File scripts/test-rinha-compliance.ps1

# Limpar dados
rm -rf data/
```

## 🐳 **Docker**

### Conformidade Rinha 2025
```bash
# Construir e executar com Docker Compose
docker-compose up --build

# Executar apenas um serviço
docker-compose up api-gateway-1
```

**Características:**
- ✅ Porta 9999 exposta
- ✅ 2 instâncias do API Gateway
- ✅ Limites de recursos: 1.5 CPU, 350MB RAM
- ✅ Rede externa payment-processor
- ✅ Build para linux/amd64

## 📈 **Monitoramento**

### Logs dos Serviços
- **API Gateway**: `api-gateway.log`
- **Payment Orchestrator**: `payment-orchestrator.log`
- **Summary Service**: `summary-service.log`
- **Processadores Mock**: `mock-default.log`, `mock-fallback.log`

### Métricas Disponíveis
- Total de pagamentos processados
- Taxa de sucesso por processador
- Tempo de resposta médio
- Uso de recursos do banco

## 🔒 **Segurança Avançada**

### Certificados mTLS
- **CA**: `certs/ca.crt`
- **Servidor**: `certs/server.crt`, `certs/server.key`
- **Cliente**: `certs/client.crt`, `certs/client.key`

### Chaves Signet-go
- **Configuração**: `config/keys.json`
- **Algoritmo**: Ed25519
- **Cache**: 5 minutos
- **Audience**: Específico por serviço

## 🎯 **Status Final**

**✅ PROJETO 100% CONFORME COM RINHA DE BACKEND 2025**

Todas as 5 histórias do épico foram concluídas e o projeto está **totalmente aderente** aos requisitos da Rinha de Backend 2025:

- ✅ **Endpoints**: Formato exato conforme especificação
- ✅ **Integração**: Payload correto para Payment Processors
- ✅ **Docker**: Compose conforme requisitos
- ✅ **Validações**: UUID e campos obrigatórios
- ✅ **Testes**: Cobertura completa de conformidade

**Pronto para submissão oficial!** 🏆

---

## 📄 **Licença**

Este projeto foi desenvolvido para a **Rinha de Backend 2025** como uma demonstração de arquitetura de microsserviços com foco em segurança, confiabilidade e **conformidade total** com os requisitos do desafio.

**Arquitetura Sênior em Golang e Microsserviços** 🏆