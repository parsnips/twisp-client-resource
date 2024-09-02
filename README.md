Twisp Client Custom Resource
-------------------------------

Twisp has the ability to exchange IAM tokens for OIDC, thereby allowing AWS identities to have permissions directly in Twisp.

This is a convenience lambda you can install in your system that can create [Twisp Clients](https://www.twisp.com/docs/reference/graphql/mutations#create-client) on behalf of other identities that will use Twisp.


## Usage


1. package up `cmd/resource` as a AWS lambda and deploy using your preferred tooling for each region you operate in.  

2. create a client in the Twisp console with sufficient privileges to create clients, in each tenant & region you operate in.  

```graphql
mutation InfraClientCreator(
  $principal: String! = "<role arn for cmd/resource lambda>"
) {
    auth {
        createClient(
            input: {
                principal: $principal
                policies: [
                    {
                        effect: ALLOW
                        actions: [SELECT, DELETE, UPDATE, INSERT]
                        resources: ["system.Client.*"]
                    }
                ]
            }
        ) { 
            principal
        }
    }
}
```

3. You may now use this as an [AWS custom resource](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-custom-resources.html) in your CDK/Cloudformation to create Twisp Clients for other entities.   

```yaml
Resources:
    CreateClientForLambdaA:
        Type: 'Custom:TwispClientCreator'
        Properties:
            # The custom resource lambda ARN, perhaps via parameter
            ServiceToken:
                Ref: 'TwispCreatorLambdaArn' 
            # x-twisp-account-id
            AccountId: 'prod'
            Region: !Sub ${AWS::Region}
            Client:
                # Lambda Role to create client for
                principal: !Sub ${LambdaARole.Arn}
                name: 'lambda A client'
                policies:
                    - effect: "ALLOW"
                      action: ["INSERT", "UPDATE", "SELECT"]
                      resources: ["financial.*"]
                      assertions:
                          isTrue: true
```
