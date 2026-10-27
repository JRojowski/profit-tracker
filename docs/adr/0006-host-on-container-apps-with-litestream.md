# Host on Azure Container Apps with SQLite and Litestream

The tool runs on Azure in the private subscription of the user, in the region Poland Central. One container app runs on the Consumption plan and scales to zero replicas when nobody uses it. The database stays one SQLite file on the disk of the container. Litestream copies each change of that file to Azure Blob Storage, and restores the file when a new replica starts.

## Why

- The tool has one user. Most of the day nobody uses it. A replica that runs all day costs money for no work.
- The Consumption plan gives each subscription 180,000 vCPU-seconds, 360,000 GiB-seconds and 2 million requests each month at no cost. With 0.25 vCPU and 0.5 GiB, this is about 200 hours of active time each month. The expected cost of the compute is 0 PLN.
- The disk of a container is temporary. When the app scales to zero, the disk and the database file go away. The database must have a copy outside the container.
- A database server (Azure SQL, PostgreSQL) runs all the time and costs money all the time. It also needs changes to the Go code.
- Microsoft does not recommend SQLite on an SMB file share (Azure Files). The file locks and the WAL mode do not work correctly there, and the database can become corrupt.
- Litestream needs the WAL mode. The tool already opens the database in the WAL mode (`backend/internal/store/store.go`). The Go code does not change.
- A Blob Storage account for a database of less than 1 MB costs less than 1 PLN each month.

## The rule

1. The container runs two processes. `litestream replicate -exec` starts `profit-tracker` as its child process.
2. Before the start, the container runs `litestream restore`. This step gets the last copy of the database from Blob Storage.
3. The container app has `minReplicas: 0` and `maxReplicas: 1`. Two replicas must never write to one database.
4. When Azure stops the replica, it sends SIGTERM. Litestream then sends the last changes to Blob Storage and stops.
5. The container app reads and writes Blob Storage with a managed identity. No storage key is in the code or in the configuration.
6. The image of the container is in the GitHub Container Registry. Azure Container Registry Basic costs about 19.50 PLN each month, and this is more than all other parts together.
7. The built-in authentication of Container Apps (Easy Auth) lets only the account of the user open the tool. The tool has personal financial data.
8. A budget with an e-mail alert watches the cost of the subscription.

## Costs

The prices are the Azure retail prices in PLN on 2026-10-27, for Poland Central.

| Option | PLN each month |
|---|---|
| This decision: scale to zero, Litestream, GHCR | about 0 to 1 |
| The same with Azure Container Registry Basic | about 20 |
| One replica all the time (`minReplicas: 1`), at the idle rate | about 17 |
| App Service B1 Linux | about 51 |

## Consequences

- The first request after a pause is slow. A new replica must get the image, restore the database and refresh the prices. This can take some seconds.
- A replica that stops without SIGTERM can lose the changes of about the last second. The user can import the XTB export again.
- Litestream is not an Azure service. The project must update its version in the image.
- Litestream also keeps old copies of the database. The user can restore the database to an earlier time, in the retention period.

## Not in this decision

- Azure Cosmos DB for the price history. It costs about 1 to 3 PLN each month in serverless mode, but it adds a second data store and a new price store in Go. SQLite holds the price history with no problem.
- More than one user.
- The test that Yahoo Finance answers requests from Azure. This test must pass before the deployment work starts.
