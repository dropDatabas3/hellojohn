#!/bin/bash
# Migration V2 Automation Script
# Ejecuta migraciones automáticas de handlers V1 → V2

set -e

# Colores
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuración
MAX_ITERATIONS=${1:-48}  # Default: todos los handlers
MODE=${2:-interactive}   # interactive | auto | background
LOG_FILE="migration_progress.log"

echo -e "${GREEN}🤖 Migration V2 Automation Script${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Max iterations: $MAX_ITERATIONS"
echo "Mode: $MODE"
echo ""

# Función para ejecutar agente de migración
migrate_handler() {
    local handler=$1
    echo -e "${YELLOW}🔄 Migrando handler: $handler${NC}"

    # Prompt para Claude Code
    local prompt="Migra el handler V1 '$handler' a V2 siguiendo CLAUDE.md § 4:

1. Lee internal/http/v1/handlers/${handler}.go
2. Verifica si ya existe service V2 equivalente en internal/http/v2/services/
3. Si NO existe:
   - Crea DTO en internal/http/v2/dto/{domain}/
   - Crea Service en internal/http/v2/services/{domain}/
   - Crea Controller en internal/http/v2/controllers/{domain}/
4. Si SÍ existe pero falta wiring:
   - Agrega service a services/{domain}/services.go
   - Agrega controller a controllers/{domain}/controllers.go
   - Registra ruta en router/{domain}_routes.go
   - Verifica wiring en app/v2/app.go
5. Actualiza MIGRATION_LOG.md con:
   - Fecha, rutas migradas, archivos creados/editados
   - Herramientas V2 usadas, dependencias
   - Wiring verificado (checkmarks + líneas de código)
6. Commit: git add . && git commit -m \"feat(v2): migrated $handler\"

Usa herramientas V2: DAL V2, JWT V2, Email V2, Cache V2.
No generes código V1. Solo V2.
"

    # Ejecutar via Claude Code CLI (ajustar según tu setup)
    if command -v claude &> /dev/null; then
        echo "$prompt" | claude --mode=autonomous >> "$LOG_FILE" 2>&1
    else
        echo -e "${RED}❌ Claude CLI no encontrado. Instala: npm install -g @anthropic/claude-code${NC}"
        echo -e "${YELLOW}💡 Ejecuta manualmente: Copia el prompt arriba y pégalo en Claude Code${NC}"
        echo ""
        echo "Handler: $handler"
        echo "Prompt:"
        echo "$prompt"
        echo ""

        if [ "$MODE" == "interactive" ]; then
            read -p "¿Migración completada manualmente? (y/n): " confirm
            if [ "$confirm" != "y" ]; then
                echo -e "${RED}⏭️  Saltando handler${NC}"
                return 1
            fi
        fi
    fi

    echo -e "${GREEN}✅ Handler $handler migrado${NC}"
    echo ""
}

# Obtener lista de handlers pendientes
get_pending_handlers() {
    if [ ! -f "MIGRATION_LOG.md" ]; then
        echo -e "${RED}❌ MIGRATION_LOG.md no encontrado${NC}"
        exit 1
    fi

    # Extraer handlers pendientes (líneas con [ ])
    grep "\[ \]" MIGRATION_LOG.md | sed 's/.*`\(.*\)\.go`.*/\1/' | head -n "$MAX_ITERATIONS"
}

# Función principal
main() {
    echo -e "${YELLOW}📋 Obteniendo handlers pendientes...${NC}"

    mapfile -t handlers < <(get_pending_handlers)
    total=${#handlers[@]}

    if [ "$total" -eq 0 ]; then
        echo -e "${GREEN}🎉 No hay handlers pendientes! Migración completa.${NC}"
        exit 0
    fi

    echo -e "${YELLOW}Total pendientes: $total${NC}"
    echo ""

    # Iterar sobre handlers
    count=0
    for handler in "${handlers[@]}"; do
        count=$((count + 1))
        echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${YELLOW}[$count/$total] Procesando: $handler${NC}"
        echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

        if migrate_handler "$handler"; then
            echo -e "${GREEN}✅ Migrado: $handler${NC}"
        else
            echo -e "${RED}❌ Falló: $handler${NC}"

            if [ "$MODE" == "interactive" ]; then
                read -p "¿Continuar con siguiente handler? (y/n): " continue_choice
                if [ "$continue_choice" != "y" ]; then
                    echo -e "${YELLOW}⏸️  Deteniendo migración${NC}"
                    exit 0
                fi
            fi
        fi

        echo ""
        sleep 2  # Pausa entre migraciones
    done

    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}🎉 Migración completa!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Mostrar estadísticas finales
    echo ""
    echo "Estadísticas finales:"
    grep "Progreso:" MIGRATION_LOG.md || echo "Progreso: 100%"
}

# Ejecutar
main
