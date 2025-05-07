# RabbitMQ Migration using Federation

## Hallo!!!

I was reading about RabbitMQ Federation recently and thought to play with the feature to check if federation can be used to migrate RabbitMQ clients across different clusters with near zero downtime. The objective of this post is to outline the steps required to set up federation between two RabbitMQ brokers `blue`(upstream) and `green`(downstream) and test the client's behaviour while switching them back and fro between `blue` and `green` clusters. Federation allows exchanges and queues to be mirrored between RabbitMQ brokers, enabling data distribution and replication.

## Environment Setup

- Docker compose file `docker-compose.yaml` defines two RabbitMQ instances:
    - `blue`: Upstream RabbitMQ broker
    - `green`: Downstream RabbitMQ broker

- Both instances have the RabbitMQ management plugin enabled for easy monitoring and management.
- The username and password for both the RabbitMQ brokers is `admin`.

Run the follwing command from the same folder as the `docker-compose.yaml` to spin up the RabbitMQ clusters
```bash
docker-compose up -d
```

- The management UI of the `blue` cluster can be accessed via `http://localhost:15672`
- The management UI of the `green` cluster can be accessed via `http://localhost:15673`

## Step 1: Create Exchanges, bindings and Queues

For the poc, let's create an exchange called `customers` which is of type `Direct`. Also create couple of queues called `customers.us` and `customer.de`.
Let's create some bindings between `customers` exchange and the queues.

- Messages with the routing Key `us` to the `customers` exchange will be forwarded to the `customers.us` queue.
- Messages with the routing Key `de` to the `customers` exchange will be forwarded to the `customers.de` queue.

Run the following command to create this setup in the `blue` cluster. Remember these resources MUST be create only in the `blue` cluster and they will be synced later to the `green` cluster which is explained in the next section.

```bash
# Declare customers exchange
docker exec -it blue  rabbitmqadmin -u admin -p admin declare exchange name=customers type=direct
# Declare Queues
docker exec -it blue  rabbitmqadmin -u admin -p admin declare queue name=customers.us durable=true
docker exec -it blue  rabbitmqadmin -u admin -p admin declare queue name=customers.de durable=true

# Declare Bindings
docker exec -it blue  rabbitmqadmin -u admin -p admin declare binding source=customers destination=customers.us routing_key=us
docker exec -it blue  rabbitmqadmin -u admin -p admin declare binding source=customers destination=customers.de routing_key=de

```
## Step 2: Sync Cluster Settings

The settings such as username, queues, exchanges and their bindings have to copied from the `blue` server to the `green` server.

Run the following command to export the settings to a file called `blue.json`

```bash
docker exec -it blue  rabbitmqctl export_definitions blue.json --format json
docker cp blue:blue.json .
```

Run the following command to import the settings into the `green` cluster

```bash
docker cp blue.json green:blue.json
docker exec -it green  rabbitmqctl import_definitions blue.json
```

Now try to open the management console of the `green` cluster using `http://localhost:15673` and you should be able to see all the exchanges, users, queues, bindings and policies declared in the `blue` cluster.

## Step 3: Configure Federation

The `blue` and `green` clusters are configured in the docker-compose file to include the Federation plugin already. 

### 1.1 Establish Federation Link at the Green cluster

Create a Federation link using the management UI of the downstream server, `green` first.

```bash
docker exec -it green rabbitmqctl set_parameter federation-upstream federation-with-blue '{"uri":"amqp://admin:admin@blue:5672","expires":3600000, "exchange":"customers"}'
```

### 1.2 Establish Federation Link at the Blue cluster (Optional)

This step must be done only when you need the Bidirectional data flow. By having Bidirectional data flow, clients can switch back to the `blue` cluster if they find something doesn't work in the `green` cluster.

Repeat the steps in the management UI of `blue` cluster as mentioned in the section 1.1 but with the following config values, 

```bash
docker exec -it blue rabbitmqctl set_parameter federation-upstream federation-with-green '{"uri":"amqp://admin:admin@green:5672","expires":3600000, "exchange":"customers"}'
```

## Step 4: Create Federation Policies

A federation policy is required in RabbitMQ to define how federation should behave for specific exchanges or queues. While federation itself sets up the communication and replication between brokers, the federation policy governs how the replicated entities are managed.

Here's why a federation policy is necessary:

- **Control Over Replication Behavior:** A federation policy allows you to specify which exchanges or queues should be federated and how they should be handled. This includes defining rules for message replication, such as what exchanges to replicate, how often, and under what conditions.
- **Selective Federation:** Not all exchanges or queues may need to be federated. With a federation policy, you can selectively choose which entities to replicate across brokers based on predefined criteria.
- **Priority and Ordering:** Federation policies enable you to set priorities for federated entities. For example, you might want certain exchanges or queues to be replicated with higher priority than others. Additionally, you can specify the order in which exchanges or queues are replicated. 

Run the following command to setup Federation policy in the `green` cluster. Our policy will be bit relaxed to include all the exchanges and queues for federation.

```bash
docker exec -it green  rabbitmqctl set_policy --vhost "/" --apply-to "all" federation ".*" '{"federation-upstream-set": "all"}'
```

If you had setup Federation in the `blue` cluster in the previous step, run the following command to set the policy, 

```bash
docker exec -it blue rabbitmqctl set_policy --vhost "/" --apply-to "all" federation ".*" '{"federation-upstream-set": "all"}'
```

## Step 5: Test with the Publisher and Consumer

### Step 5.1: Run the Publisher to the Blue Cluster

The publisher code is present inside the `publisher` folder in the repository. Run the following command to run the publisher to publish 2500 messages equally to `customers.us` and `customers.de` queues in the `blue` cluster.

```bash
cd publisher && go run main.go --cluster=blue
```

### Step 5.2: Run the Consumer to the Blue cluster

The consumer code is present inside the `consumer` folder in the repository. Run the following command to run the consumer and consume messages from the above queues in the blue cluster.

```bash
cd consumer && go run main.go --cluster=blue --queue=customers.us
```

The consumer will start consuming the messages from the `customers.us` queue. Stop the consumer after few minutes so that it does not consume all the messages.

![Blue Consumer](./assets/consumer-blue.png)

### Step 5.3: Run the Consumer in the Green cluster

Once the consumer is stopped in the `blue` cluster, start the consumer back in the `green` cluster by the following command,

```bash
cd consumer && go run main.go --cluster=green --queue=customers.us
```

![Green Consumer](./assets/consumer-green.png)

The consumer consumes messages in the `green` cluster from where it left off in the `blue` cluster. This shows that federation works in a forward direction.

### Step 5.4: Run the Publisher in the Green cluster

It's time to test the Federation in a reverse direction. Let's point our publisher to publish message in the `green` cluster.

```bash
cd publisher && go run main.go --cluster=green
```

The consumer which is running in the `green` cluster can consume the new messages from the publisher. If the consumer is stopped and started in the `blue` cluster, then the messages are relayed from `green` to `blue` and are delivered to the consumer.

## Conclusion

Thus **Federation** is a great mechanism to move RabbitMQ clients across different clusters. This feature becomes super useful when migrating the RabbitMQ clusters. However, an important note is Federation do not relay messages which are published to the **default** exchange.

