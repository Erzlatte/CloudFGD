NAME     = CloudFGD
COMPOSE  = docker compose -f srcs/docker-compose.yml --env-file srcs/.env
DATA_DIR = $(HOME)/data

all: up

up:
	@mkdir -p $(DATA_DIR)/app $(DATA_DIR)/certs
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) up -d --build

down:
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) down

stop:
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) stop

start:
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) start

logs:
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) logs -f

ps:
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) ps

health:
	@for c in app proxy; do \
		printf "%-6s %s\n" $$c "$$(docker inspect -f '{{.State.Health.Status}}' $$c 2>/dev/null || echo 'no existe')"; \
	done

clean: down
	@docker system prune -af

fclean: down
	@DATA_DIR=$(DATA_DIR) $(COMPOSE) down -v --rmi all
	@sudo rm -rf $(DATA_DIR)/app $(DATA_DIR)/certs

re: fclean all

.PHONY: all up down stop start logs ps health clean fclean re
