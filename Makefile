# Creates (or updates) secrets object on the k8s cluster server
upsert-secrets:
	kubectl apply -n taina-backend -f secrets/secrets.yml


run-local:
	go run ./news.go ./bot.go ./main.go
