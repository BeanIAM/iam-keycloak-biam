package net.bean.keycloak.kafka.it;

import dasniko.testcontainers.keycloak.KeycloakContainer;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.clients.consumer.ConsumerRecords;
import org.apache.kafka.clients.consumer.KafkaConsumer;
import org.apache.kafka.common.serialization.StringDeserializer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.keycloak.admin.client.Keycloak;
import org.keycloak.admin.client.KeycloakBuilder;
import org.keycloak.admin.client.resource.RealmResource;
import org.keycloak.admin.client.resource.UserResource;
import org.keycloak.representations.idm.CredentialRepresentation;
import org.keycloak.representations.idm.RealmEventsConfigRepresentation;
import org.keycloak.representations.idm.UserRepresentation;
import org.testcontainers.containers.KafkaContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import org.testcontainers.utility.MountableFile;

import javax.ws.rs.core.Response;
import java.time.Duration;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Properties;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

@Testcontainers
public class KeycloakKafkaIT {
    //    private static final String KEYCLOAK_IMAGE = "quay.io/keycloak/keycloak:21.0.1";
    private static final int KEYCLOAK_PORT = 8080;

    private static final String KAFKA_TOPIC = "keycloak-events";
    private static final String KAFKA_ADMIN_TOPIC = "keycloak-admin-events";

    private static final String KEYCLOAK_USERNAME = "admin";
    private static final String KEYCLOAK_PASSWORD = "admin";

    private static final String KEYCLOAK_REALM_NAME = "test-realm";
    private static final String KEYCLOAK_CLIENT_ID = "keycloak";
    private static final String KEYCLOAK_CLIENT_SECRET = "admin";

    private static Keycloak keycloakClient;
    private static RealmResource realmResource;

    private static final DockerImageName KAFKA_IMAGE_NAME = DockerImageName.parse("confluentinc/cp-kafka:5.5.1");

    private static final String JAR_NAME = "kafka-spi-1.0.1-jar-with-dependencies.jar";
    static Network network = Network.newNetwork();

    public static final KafkaContainer kafkaContainer = new KafkaContainer(KAFKA_IMAGE_NAME)
        .withNetwork(network)
        .withStartupTimeout(Duration.ofMinutes(2))
        .withEmbeddedZookeeper()
        .withExposedPorts(9093)
        .withEnv("KAFKA_CLIENT_ID", "keycloak")
        .withCreateContainerCmdModifier(cmd -> cmd.withHostName("kafka"))
        ;

    private static final KeycloakContainer keycloakContainer = new KeycloakContainer("quay.io/keycloak/keycloak:21.0.1")
        .withExposedPorts(KEYCLOAK_PORT)
        .withNetwork(network)
        .withStartupTimeout(Duration.ofMinutes(2))
        .withCopyFileToContainer(
            MountableFile.forClasspathResource(JAR_NAME),
            "/opt/keycloak/providers/" + JAR_NAME
        )
        .withEnv("KEYCLOAK_USER", KEYCLOAK_USERNAME)
        .withEnv("KEYCLOAK_PASSWORD", KEYCLOAK_PASSWORD)
        .withEnv("DB_VENDOR", "H2")
        .withEnv("KAFKA_CLIENT_ID", KEYCLOAK_CLIENT_ID)
        .withEnv("KAFKA_TOPIC", KAFKA_TOPIC)
        .withEnv("KAFKA_ADMIN_TOPIC", KAFKA_ADMIN_TOPIC)
        ;

    @BeforeAll
    public static void setUp() throws InterruptedException {
        // Set DOCKER_HOST environment variable for Testcontainers
        System.setProperty("DOCKER_HOST", "tcp://localhost:2375");
        kafkaContainer.start();
        // Wait for Kafka to start up
        Thread.sleep(15000);

        keycloakContainer
            .withEnv("KAFKA_BOOTSTRAP_SERVERS", "kafka:9092");
        keycloakContainer.start();

        // Wait for Keycloak to start up
        assertTrue(keycloakContainer.isRunning());

        // Get admin access token
        String accessToken = KeycloakBuilder.builder()
            .serverUrl(keycloakContainer.getAuthServerUrl())
            .realm("master")
            .clientId("admin-cli")
            .username("admin")
            .password("admin")
            .build()
            .tokenManager()
            .getAccessToken()
            .getToken();

        // Create Keycloak client with admin access token
        keycloakClient = KeycloakBuilder.builder()
            .serverUrl(keycloakContainer.getAuthServerUrl())
            .realm("master")
            .clientId("admin-cli")
            .authorization(accessToken)
            .build();
        realmResource = keycloakClient.realm("master");

        // Setup kafka event listener
        RealmEventsConfigRepresentation config = realmResource.getRealmEventsConfig();
        config.setAdminEventsEnabled(true);
        config.setAdminEventsDetailsEnabled(true);
        config.setEventsEnabled(true);
        config.setEventsExpiration(3600L);
        List<String> events = config.getEventsListeners();
        events.add("kafka");
        config.setEventsListeners(events);
//        System.out.println("enabled event types");
//        List<String> enabledEventTypes = config.getEnabledEventTypes();
//        enabledEventTypes.add("ADMIN_UPDATE");
//        System.out.println(config.getEnabledEventTypes());
        realmResource.updateRealmEventsConfig(config);
    }

    @AfterEach
    public void tearDown() {
        keycloakContainer.close();
        kafkaContainer.close();
    }

    @Test
    public void whenUpdateUserThenKafkaEventIsPublished() {
        UserRepresentation user = new UserRepresentation();
        CredentialRepresentation credentialRepresentation = new CredentialRepresentation();
        credentialRepresentation.setType(CredentialRepresentation.PASSWORD);
        credentialRepresentation.setValue("testpassword");
        user.setId("test123");
        user.setUsername("testuser");
        user.setEmail("testuser@test.com");
        user.setFirstName("Test");
        user.setLastName("User");
        user.setEnabled(true);
        user.setCredentials(Arrays.asList(credentialRepresentation));

        Response response = realmResource.users().create(user);
        assertEquals(201, response.getStatus());

        // Verify that the user was updated
        UserRepresentation adminUser = realmResource.users().searchByUsername("admin", true).get(0);
        UserResource userResource = realmResource.users().get(adminUser.getId());
        UserRepresentation updatedUser = userResource.toRepresentation();
        updatedUser.setFirstName("John");
        updatedUser.setLastName("Doe");
        updatedUser.setEmail("nhuthm080280@gmail.com");

        userResource.update(updatedUser);

        UserRepresentation savedAdminUser = realmResource.users().searchByUsername("admin", true).get(0);
        assertEquals("John", savedAdminUser.getFirstName());
        assertEquals("Doe", savedAdminUser.getLastName());


        // Set up a Kafka consumer
        Properties consumerProps = new Properties();
        consumerProps.put(ConsumerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaContainer.getBootstrapServers());
        consumerProps.put(ConsumerConfig.GROUP_ID_CONFIG, "test-group");
        consumerProps.put(ConsumerConfig.KEY_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.VALUE_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
        KafkaConsumer<String, String> keycloakAdminEventconsumer = new KafkaConsumer<>(consumerProps);

        // Subscribe to the Kafka topic and consume the message
        keycloakAdminEventconsumer.subscribe(Collections.singletonList(KAFKA_ADMIN_TOPIC));
        ConsumerRecords<String, String> consumerAdminEventRecords = keycloakAdminEventconsumer.poll(Duration.ofSeconds(10));
        Assertions.assertNotNull(consumerAdminEventRecords);

        assertFalse(consumerAdminEventRecords.isEmpty());
        consumerAdminEventRecords.forEach(e -> {
            System.out.println("Keycloak Admin events");
            System.out.println(e);
        });
        Assertions.assertEquals(consumerAdminEventRecords.count(), 3);

        // keycloak event consumer
        KafkaConsumer<String, String> keycloakEventConsumer = new KafkaConsumer<>(consumerProps);
        keycloakEventConsumer.subscribe(Collections.singletonList(KAFKA_TOPIC));
        ConsumerRecords<String, String> consumerEventRecords = keycloakEventConsumer.poll(Duration.ofSeconds(10));
        consumerEventRecords.forEach(e -> {
            System.out.println("Keycloak events");
            System.out.println(e);
        });
    }

    /* @Test
    @Disabled
    public void testKafka() throws InterruptedException {
        // Set up a Kafka producer
        Properties producerProps = new Properties();
        producerProps.put(ProducerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaContainer.getBootstrapServers());
        producerProps.put(ProducerConfig.KEY_SERIALIZER_CLASS_CONFIG, StringSerializer.class);
        producerProps.put(ProducerConfig.VALUE_SERIALIZER_CLASS_CONFIG, StringSerializer.class);
        Producer<String, String> producer = new KafkaProducer<>(producerProps);

        // Produce a message to the Kafka topic
        String message = "Hello, Kafka!";
        producer.send(new ProducerRecord<>(KAFKA_ADMIN_TOPIC, message));

        // Set up a Kafka consumer
        Properties consumerProps = new Properties();
        consumerProps.put(ConsumerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaContainer.getBootstrapServers());
        consumerProps.put(ConsumerConfig.GROUP_ID_CONFIG, "test-group");
        consumerProps.put(ConsumerConfig.KEY_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.VALUE_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);
        consumerProps.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
        KafkaConsumer<String, String> consumer = new KafkaConsumer<>(consumerProps);

        // Subscribe to the Kafka topic and consume the message
        consumer.subscribe(Collections.singletonList(KAFKA_ADMIN_TOPIC));
        ConsumerRecords<String, String> consumerRecords = consumer.poll(Duration.ofSeconds(10));
        Assertions.assertNotNull(consumerRecords);
        Assertions.assertEquals(consumerRecords.iterator().next().value(), message);
    } */
}
