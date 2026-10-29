// The infrastructure of profit-tracker in the resource group rg-profit-tracker. See ADR 0006.
// The deployment workflow applies this template at each deployment, with the image tag as a parameter.

@description('The Azure region. The resource group is in Poland Central.')
param location string = resourceGroup().location

@description('The tag of the image in ghcr.io. The deployment workflow uses the commit SHA.')
param imageTag string

@description('The image name in ghcr.io, with no tag.')
param imageName string = 'ghcr.io/jrojowski/profit-tracker'

@description('The GitHub user that owns the GHCR token.')
param ghcrUsername string

@description('The classic GitHub token with only read:packages. The container app pulls the private image with it.')
@secure()
param ghcrToken string

var appName = 'profit-tracker'
var port = 8080

resource logs 'Microsoft.OperationalInsights/workspaces@2025-02-01' = {
  name: 'log-${appName}'
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
    retentionInDays: 30
    // A loop of errors cannot make a large bill.
    workspaceCapping: {
      dailyQuotaGb: json('0.1')
    }
  }
}

// No workload profiles: the environment has only the Consumption plan.
resource environment 'Microsoft.App/managedEnvironments@2025-01-01' = {
  name: 'cae-${appName}'
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: logs.properties.customerId
        sharedKey: logs.listKeys().primarySharedKey
      }
    }
  }
}

resource app 'Microsoft.App/containerApps@2025-01-01' = {
  name: appName
  location: location
  properties: {
    environmentId: environment.id
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: true
        targetPort: port
        allowInsecure: false
        transport: 'auto'
      }
      secrets: [
        {
          name: 'ghcr-token'
          value: ghcrToken
        }
      ]
      registries: [
        {
          server: 'ghcr.io'
          username: ghcrUsername
          passwordSecretRef: 'ghcr-token'
        }
      ]
    }
    template: {
      containers: [
        {
          name: appName
          image: '${imageName}:${imageTag}'
          resources: {
            cpu: json('0.25')
            memory: '0.5Gi'
          }
          probes: [
            {
              type: 'Liveness'
              httpGet: {
                path: '/api/health'
                port: port
              }
            }
            {
              type: 'Readiness'
              httpGet: {
                path: '/api/health'
                port: port
              }
            }
          ]
        }
      ]
      // Two replicas must never write to one database.
      scale: {
        minReplicas: 0
        maxReplicas: 1
      }
    }
  }
}

@description('The public HTTPS address of the tool.')
output url string = 'https://${app.properties.configuration.ingress.fqdn}'
