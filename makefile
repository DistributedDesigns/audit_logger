IMG_NAME = mysql-trading
IMG_BASE = mysql:5.7
PORT = 44432
USER = mysqladmin
PASS = mysqladmin
DB_NAME = audit
SQL_DIR = ${PWD}/sql

.PHONY: run start stop clean cli

run:
	@docker run \
		--name ${IMG_NAME} \
		--publish ${PORT}:3306 \
		--env MYSQL_RANDOM_ROOT_PASSWORD=yes \
		--env MYSQL_USER=${USER} \
		--env MYSQL_PASSWORD=${PASS} \
		--env MYSQL_DATABASE=${DB_NAME} \
		--volume ${SQL_DIR}:/docker-entrypoint-initdb.d/ \
		--detach \
		${IMG_BASE}

	@echo "MySQL: ${PORT}"

start:
	docker start ${IMG_NAME}
	@echo "MySQL: ${PORT}"

stop:
	docker stop ${IMG_NAME}

clean: stop
	docker rm ${IMG_NAME}

cli:
	@docker exec -it ${IMG_NAME} \
		mysql --user=${USER} --password=${PASS} ${DB_NAME}
