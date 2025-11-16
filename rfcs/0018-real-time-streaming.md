# RFC 0018: Real-Time Data Streaming Support

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-16  
**Updated:** 2025-01-16  

---

## Summary

Implement real-time data streaming capabilities with Kafka/Pulsar integration, Change Data Capture (CDC), streaming ingestion, GraphQL subscriptions, and event-driven triggers for live data synchronization.

**Current state:** Batch-only data ingestion, no real-time updates  
**Proposed state:** Streaming ingestion with CDC, live subscriptions, and event-driven processing

---

## Motivation

### Current Limitations

1. **Batch-only ingestion:**
   - Manual batch imports
   - Delayed data availability
   - No real-time sync
   - Polling-based updates

2. **No change notifications:**
   - Cannot notify on data changes
   - No event-driven workflows
   - Missing real-time use cases

3. **Polling overhead:**
   - Clients poll for updates
   - Inefficient resource usage
   - High latency for changes

### Use Cases

**Real-time recommendations:**
- Update product recommendations on inventory changes
- Personalization based on live user behavior
- Trending content detection

**Live dashboards:**
- Real-time analytics
- Monitoring dashboards
- Business intelligence

**Event-driven architectures:**
- Microservice synchronization
- Data pipeline orchestration
- Real-time ETL

---

## Detailed Design

### Kafka Integration

```go
type KafkaConsumer struct {
    consumer   *kafka.Consumer
    vectorizer Vectorizer
    writer     *BatchWriter
    config     *KafkaConfig
}

type KafkaConfig struct {
    Brokers       []string
    Topics        []string
    GroupID       string
    
    // Processing
    BatchSize     int
    FlushInterval time.Duration
    
    // Mapping
    ClassMapping  map[string]ClassConfig
}

type ClassConfig struct {
    ClassName      string
    KeyField       string
    VectorizeFields []string
    TransformFunc  func(map[string]interface{}) (*Object, error)
}

func (k *KafkaConsumer) Start(ctx context.Context) error {
    // Subscribe to topics
    if err := k.consumer.SubscribeTopics(k.config.Topics, nil); err != nil {
        return err
    }
    
    batch := make([]*Object, 0, k.config.BatchSize)
    ticker := time.NewTicker(k.config.FlushInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return k.flush(batch)
            
        case <-ticker.C:
            if len(batch) > 0 {
                if err := k.flush(batch); err != nil {
                    return err
                }
                batch = batch[:0]
            }
            
        default:
            // Poll for messages
            msg, err := k.consumer.ReadMessage(100 * time.Millisecond)
            if err != nil {
                continue
            }
            
            // Transform message to object
            obj, err := k.transform(msg)
            if err != nil {
                log.Errorf("Failed to transform message: %v", err)
                continue
            }
            
            // Add to batch
            batch = append(batch, obj)
            
            // Flush if batch full
            if len(batch) >= k.config.BatchSize {
                if err := k.flush(batch); err != nil {
                    return err
                }
                batch = batch[:0]
            }
            
            // Commit offset
            k.consumer.CommitMessage(msg)
        }
    }
}

func (k *KafkaConsumer) transform(msg *kafka.Message) (*Object, error) {
    // Parse JSON
    var data map[string]interface{}
    if err := json.Unmarshal(msg.Value, &data); err != nil {
        return nil, err
    }
    
    // Get class config
    topic := *msg.TopicPartition.Topic
    classConfig := k.config.ClassMapping[topic]
    
    // Extract ID
    id := data[classConfig.KeyField].(string)
    
    // Vectorize if needed
    var vector []float32
    if len(classConfig.VectorizeFields) > 0 {
        text := k.extractText(data, classConfig.VectorizeFields)
        vector, err = k.vectorizer.Vectorize(context.Background(), text)
        if err != nil {
            return nil, err
        }
    }
    
    // Transform to object
    obj := &Object{
        ID:         UUID(id),
        Class:      classConfig.ClassName,
        Properties: data,
        Vector:     vector,
    }
    
    // Apply custom transformation
    if classConfig.TransformFunc != nil {
        return classConfig.TransformFunc(data)
    }
    
    return obj, nil
}
```

### Change Data Capture (CDC)

```go
type CDCPublisher struct {
    kafka      *kafka.Producer
    config     *CDCConfig
    filters    []CDCFilter
}

type CDCConfig struct {
    Enabled    bool
    Topic      string
    Format     string  // json | avro | protobuf
    
    // Filters
    Classes    []string  // Only publish changes for these classes
    Operations []string  // INSERT, UPDATE, DELETE
}

type ChangeEvent struct {
    Type       ChangeType
    Class      string
    ID         UUID
    Before     *Object
    After      *Object
    Timestamp  time.Time
    UserID     UUID
    
    // Metadata
    TransactionID *TxID
    SequenceNum   uint64
}

// Hook into write operations
func (p *CDCPublisher) OnWrite(ctx context.Context, event *ChangeEvent) error {
    // Apply filters
    if !p.shouldPublish(event) {
        return nil
    }
    
    // Serialize event
    data, err := p.serialize(event)
    if err != nil {
        return err
    }
    
    // Publish to Kafka
    return p.kafka.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{
            Topic:     &p.config.Topic,
            Partition: kafka.PartitionAny,
        },
        Key:   []byte(event.ID),
        Value: data,
        Headers: []kafka.Header{
            {Key: "type", Value: []byte(event.Type)},
            {Key: "class", Value: []byte(event.Class)},
        },
    }, nil)
}

// Debezium-style CDC format
type DebeziumEvent struct {
    Before  map[string]interface{} `json:"before"`
    After   map[string]interface{} `json:"after"`
    Source  SourceMetadata         `json:"source"`
    Op      string                 `json:"op"` // c, u, d, r
    TsMs    int64                  `json:"ts_ms"`
}
```

### GraphQL Subscriptions

```go
// Subscription manager
type SubscriptionManager struct {
    subscribers map[string][]*Subscriber
    mu          sync.RWMutex
    cdc         *CDCPublisher
}

type Subscriber struct {
    ID      string
    Filter  *Filter
    Channel chan *ChangeEvent
}

func (m *SubscriptionManager) Subscribe(
    ctx context.Context,
    filter *Filter,
) (<-chan *ChangeEvent, error) {
    sub := &Subscriber{
        ID:      generateID(),
        Filter:  filter,
        Channel: make(chan *ChangeEvent, 100),
    }
    
    m.mu.Lock()
    m.subscribers[filter.Class] = append(m.subscribers[filter.Class], sub)
    m.mu.Unlock()
    
    // Cleanup on context cancellation
    go func() {
        <-ctx.Done()
        m.unsubscribe(sub)
    }()
    
    return sub.Channel, nil
}

func (m *SubscriptionManager) Publish(event *ChangeEvent) {
    m.mu.RLock()
    subs := m.subscribers[event.Class]
    m.mu.RUnlock()
    
    for _, sub := range subs {
        if sub.Filter.Matches(event) {
            select {
            case sub.Channel <- event:
            case <-time.After(100 * time.Millisecond):
                // Slow subscriber, drop event
                log.Warnf("Dropped event for slow subscriber %s", sub.ID)
            }
        }
    }
}
```

**GraphQL Subscription Example:**

```graphql
subscription {
  articleChanges(
    where: { category: { equals: "AI" } }
  ) {
    type  # INSERT, UPDATE, DELETE
    
    # Object data
    article {
      id
      title
      content
      publishedAt
    }
    
    # Change metadata
    _metadata {
      timestamp
      user
      transactionId
    }
  }
}
```

### Event-Driven Triggers

```go
type TriggerEngine struct {
    triggers []Trigger
    executor *TriggerExecutor
}

type Trigger struct {
    Name        string
    Description string
    
    // Condition
    Class       string
    Event       EventType
    Filter      *Filter
    
    // Action
    Action      TriggerAction
    Config      map[string]interface{}
}

type TriggerAction interface {
    Execute(ctx context.Context, event *ChangeEvent) error
}

// Example: Webhook trigger
type WebhookAction struct {
    URL     string
    Method  string
    Headers map[string]string
}

func (a *WebhookAction) Execute(ctx context.Context, event *ChangeEvent) error {
    payload, _ := json.Marshal(event)
    
    req, err := http.NewRequestWithContext(ctx, a.Method, a.URL, bytes.NewReader(payload))
    if err != nil {
        return err
    }
    
    for k, v := range a.Headers {
        req.Header.Set(k, v)
    }
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook failed with status %d", resp.StatusCode)
    }
    
    return nil
}
```

---

## Configuration

```yaml
# Kafka consumer
streaming:
  kafka:
    enabled: true
    brokers: ["kafka-1:9092", "kafka-2:9092"]
    
    consumers:
      - topics: ["products"]
        groupId: "weaviate-products"
        class: "Product"
        
        # Field mapping
        mapping:
          keyField: "product_id"
          vectorizeFields: ["name", "description"]
          
        # Performance
        batchSize: 1000
        flushInterval: "1s"

# CDC publishing
cdc:
  enabled: true
  
  kafka:
    topic: "weaviate.changes"
    format: "debezium"
    
  # Filters
  filters:
    classes: ["Article", "Product"]
    operations: ["INSERT", "UPDATE", "DELETE"]

# Subscriptions
subscriptions:
  enabled: true
  maxSubscribers: 10000
  bufferSize: 100
```

---

## Performance Impact

### Throughput

| Metric | Batch | Streaming | Improvement |
|--------|-------|-----------|-------------|
| Ingestion latency | 10s | 100ms | 99% faster |
| Data freshness | Minutes | Seconds | 60x better |
| Throughput | 10k obj/s | 50k obj/s | 5x higher |

### Resource Usage

| Component | CPU | Memory | Network |
|-----------|-----|--------|---------|
| Kafka consumer | +10% | +200MB | Moderate |
| CDC publisher | +5% | +100MB | Low |
| Subscriptions | +15% | +500MB | High |

---

## Implementation Plan

### Phase 1: Kafka Integration (4 weeks)
- [ ] Kafka consumer
- [ ] Message transformation
- [ ] Batch processing
- [ ] Error handling

### Phase 2: CDC (3 weeks)
- [ ] Change detection
- [ ] Event publishing
- [ ] Debezium format
- [ ] Filtering

### Phase 3: Subscriptions (3 weeks)
- [ ] Subscription manager
- [ ] GraphQL subscriptions
- [ ] WebSocket transport
- [ ] Testing

### Phase 4: Triggers (2 weeks)
- [ ] Trigger engine
- [ ] Webhook actions
- [ ] Documentation
- [ ] Examples

**Total: 12 weeks**

---

## Success Criteria

- ✅ <100ms end-to-end latency
- ✅ 50k objects/second throughput
- ✅ Support for Kafka and Pulsar
- ✅ GraphQL subscriptions working
- ✅ CDC with Debezium format

---

## References

- Kafka Connect: https://kafka.apache.org/documentation/#connect
- Debezium: https://debezium.io/
- GraphQL Subscriptions: https://www.apollographql.com/docs/react/data/subscriptions/

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-16*