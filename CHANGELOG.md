# CHANGE LOGS

### 5.3.0

- Support Session and Transaction
  - Added `Session` struct for managing database sessions with unique session IDs
  - Added `Transaction` struct for transaction management within sessions
  - Session supports both UQL and GQL execution
  - Transaction supports GQL execution with commit and rollback operations
  - Configurable session timeout and graph settings
  - NOT thread-safe - caller must synchronize if used across goroutines
  - Comprehensive test coverage with examples

- Support POINT3D data type
  - Added `types.Point3D` struct with X, Y, Z coordinates
  - String-based serialization following POINT pattern
  - Comprehensive test coverage for insert and retrieval operations

- Support RECORD data type
  - Added `types.Record` struct for storing arbitrary JSON data
  - Handles JSON objects, arrays, and nested structures
  - Automatic array wrapping for root-level JSON arrays (workaround for database behavior)
  - Provides convenient methods: `Get()`, `Set()`, `Has()`, `Keys()`, `ToMap()`, `ToJSONString()`
  - Full serialization/deserialization support
  - Comprehensive test coverage with GQL queries

### 4.4.0

- Add Graph result type, Change original DateItem.asGraphs() method to DateItem.asGraphInfos() and return
  List<GraphInfo>, new asGraphs() will return List<Graph> for the new Graph result type.

### 4.3.5

- fix

### 4.3.4

- remove backup method
- add encrypt attribute for property.

### 4.3.3

- Support set data type.

### 4.3.2

- Support decimal data type.

### 4.3.1

- Support blob data type.
- Add passwordEncrypt option for configuration to encrypt password. Available values are MD5, LDAP, NOTHING, MD5 is
  default.
- Add read and write properties for Ultipa property for server > 4.3.64
- Send grant().system(), grant().privilege, grant().node_privilege and grant().edge_privilege to global graph set.

### 4.3.0

- Support data type LIST and POINT, remove ARRAY data type.
- AsAttr support LIST data type.
- Support NULL value.
- Implement backup interface.
- Improve error message when failed to create connection pool
- Differentiate timestamp and datetime when parse from string
- Support special character when create/show schema and create property
-

## Version 4.2.1

- fixed bug: clear task should send to global graphset

## Version 4.2.0

- add InstallExta Methods
- add UninstallExta Methods

## Version 4.0.10

- add insertRequestConfig
- add insertResponse
- update insertBatch* Methods

## Version 4.0.5

- Support more layouts when parsing string to UltipaTime
- Support timestamp when deserializing from bytes to GoLang Type and return to an interface
- Attr add PropertyType
- Add more comments to request configurations and connection configurations

## Version 4.0.4 - release

- Add Model Manager
    - manage model(graph)
        - manage schemas for a model
- Add load db config from yaml
- Add asFirstNode、asFirstEdge Methods for DataItem
- Bug Fix
    - Fix Context Memory leak