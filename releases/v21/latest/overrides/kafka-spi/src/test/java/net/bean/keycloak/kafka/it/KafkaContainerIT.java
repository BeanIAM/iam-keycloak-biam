package net.bean.keycloak.kafka.it;

import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.clients.consumer.ConsumerRecords;
import org.apache.kafka.clients.consumer.KafkaConsumer;
import org.apache.kafka.clients.producer.KafkaProducer;
import org.apache.kafka.clients.producer.Producer;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.apache.kafka.common.serialization.StringDeserializer;
import org.apache.kafka.common.serialization.StringSerializer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.KafkaContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

import java.time.Duration;
import java.util.Collections;
import java.util.Properties;

@Testcontainers
public class KafkaContainerIT {


    private static final DockerImageName KAFKA_IMAGE_NAME = DockerImageName.parse("confluentinc/cp-kafka:5.5.1");
    @Container
    public static KafkaContainer kafkaContainer = new KafkaContainer()
        .withNetworkAliases("backend")
        .withStartupTimeout(Duration.ofSeconds(60))
        .withEmbeddedZookeeper()
        .withExposedPorts(9093)
        ;

    @BeforeEach
    public void setUp() throws InterruptedException {
        kafkaContainer.start();
    }

    @AfterEach
    public void tearDown() {
        kafkaContainer.close();
    }

    private static final String KAFKA_TOPIC_1 = "test-topic";
    private static final String KAFKA_BOOTSTRAP_SERVERS = "localhost:9092";

    @Test
    public void testKafka() throws InterruptedException {
        // Set up a Kafka producer
        Properties producerProps = new Properties();
        producerProps.put(ProducerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaContainer.getBootstrapServers());
        producerProps.put(ProducerConfig.KEY_SERIALIZER_CLASS_CONFIG, StringSerializer.class);
        producerProps.put(ProducerConfig.VALUE_SERIALIZER_CLASS_CONFIG, StringSerializer.class);
        Producer<String, String> producer = new KafkaProducer<>(producerProps);

        // Produce a message to the Kafka topic
        String message = "Hello, Kafka!";
        producer.send(new ProducerRecord<>(KAFKA_TOPIC_1, message));

        // Set up a Kafka consumer
        Properties consumerProps = new Properties();
        consumerProps.put(ConsumerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaContainer.getBootstrapServers());
        consumerProps.put(ConsumerConfig.GROUP_ID_CONFIG, "test-group");
        consumerProps.put(ConsumerConfig.KEY_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.VALUE_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
        KafkaConsumer<String, String> consumer = new KafkaConsumer<>(consumerProps);

        // Subscribe to the Kafka topic and consume the message
        consumer.subscribe(Collections.singletonList(KAFKA_TOPIC_1));
        ConsumerRecords<String, String> consumerRecords = consumer.poll(Duration.ofSeconds(10));
        Assertions.assertNotNull(consumerRecords);
        Assertions.assertEquals(consumerRecords.iterator().next().value(), message);
    }
}
