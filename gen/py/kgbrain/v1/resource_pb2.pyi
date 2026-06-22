from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DatabaseType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DATABASE_TYPE_UNSPECIFIED: _ClassVar[DatabaseType]
    DATABASE_TYPE_POSTGRES: _ClassVar[DatabaseType]
DATABASE_TYPE_UNSPECIFIED: DatabaseType
DATABASE_TYPE_POSTGRES: DatabaseType

class SetLLMResourceRequest(_message.Message):
    __slots__ = ("resource_id", "name", "config")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: LLMResourceConfig
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[LLMResourceConfig, _Mapping]] = ...) -> None: ...

class SetLLMResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class GetLLMResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class GetLLMResourceResponse(_message.Message):
    __slots__ = ("resource_id", "name", "config", "created_at_unix", "updated_at_unix")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: LLMResourceConfig
    created_at_unix: int
    updated_at_unix: int
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[LLMResourceConfig, _Mapping]] = ..., created_at_unix: _Optional[int] = ..., updated_at_unix: _Optional[int] = ...) -> None: ...

class DeleteLLMResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class DeleteLLMResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class SetEmbeddingResourceRequest(_message.Message):
    __slots__ = ("resource_id", "name", "config")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: EmbeddingResourceConfig
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[EmbeddingResourceConfig, _Mapping]] = ...) -> None: ...

class SetEmbeddingResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class GetEmbeddingResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class GetEmbeddingResourceResponse(_message.Message):
    __slots__ = ("resource_id", "name", "config", "created_at_unix", "updated_at_unix")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: EmbeddingResourceConfig
    created_at_unix: int
    updated_at_unix: int
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[EmbeddingResourceConfig, _Mapping]] = ..., created_at_unix: _Optional[int] = ..., updated_at_unix: _Optional[int] = ...) -> None: ...

class DeleteEmbeddingResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class DeleteEmbeddingResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class LLMResourceConfig(_message.Message):
    __slots__ = ("base_url", "api_key", "model", "timeout_seconds", "temperature", "max_concurrency")
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    API_KEY_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_SECONDS_FIELD_NUMBER: _ClassVar[int]
    TEMPERATURE_FIELD_NUMBER: _ClassVar[int]
    MAX_CONCURRENCY_FIELD_NUMBER: _ClassVar[int]
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    temperature: float
    max_concurrency: int
    def __init__(self, base_url: _Optional[str] = ..., api_key: _Optional[str] = ..., model: _Optional[str] = ..., timeout_seconds: _Optional[int] = ..., temperature: _Optional[float] = ..., max_concurrency: _Optional[int] = ...) -> None: ...

class EmbeddingResourceConfig(_message.Message):
    __slots__ = ("base_url", "api_key", "model", "timeout_seconds", "max_concurrency")
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    API_KEY_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_SECONDS_FIELD_NUMBER: _ClassVar[int]
    MAX_CONCURRENCY_FIELD_NUMBER: _ClassVar[int]
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    max_concurrency: int
    def __init__(self, base_url: _Optional[str] = ..., api_key: _Optional[str] = ..., model: _Optional[str] = ..., timeout_seconds: _Optional[int] = ..., max_concurrency: _Optional[int] = ...) -> None: ...

class SetDatabaseResourceRequest(_message.Message):
    __slots__ = ("resource_id", "name", "config")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: DatabaseResourceConfig
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[DatabaseResourceConfig, _Mapping]] = ...) -> None: ...

class SetDatabaseResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class GetDatabaseResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class GetDatabaseResourceResponse(_message.Message):
    __slots__ = ("resource_id", "name", "config", "created_at_unix", "updated_at_unix")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    name: str
    config: DatabaseResourceConfig
    created_at_unix: int
    updated_at_unix: int
    def __init__(self, resource_id: _Optional[str] = ..., name: _Optional[str] = ..., config: _Optional[_Union[DatabaseResourceConfig, _Mapping]] = ..., created_at_unix: _Optional[int] = ..., updated_at_unix: _Optional[int] = ...) -> None: ...

class DeleteDatabaseResourceRequest(_message.Message):
    __slots__ = ("resource_id",)
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    def __init__(self, resource_id: _Optional[str] = ...) -> None: ...

class DeleteDatabaseResourceResponse(_message.Message):
    __slots__ = ("resource_id", "status")
    RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    resource_id: str
    status: str
    def __init__(self, resource_id: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class DatabaseResourceConfig(_message.Message):
    __slots__ = ("type", "postgres")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    POSTGRES_FIELD_NUMBER: _ClassVar[int]
    type: DatabaseType
    postgres: PostgresResourceConfig
    def __init__(self, type: _Optional[_Union[DatabaseType, str]] = ..., postgres: _Optional[_Union[PostgresResourceConfig, _Mapping]] = ...) -> None: ...

class PostgresResourceConfig(_message.Message):
    __slots__ = ("host", "port", "database", "user", "password", "sslmode")
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    SSLMODE_FIELD_NUMBER: _ClassVar[int]
    host: str
    port: int
    database: str
    user: str
    password: str
    sslmode: str
    def __init__(self, host: _Optional[str] = ..., port: _Optional[int] = ..., database: _Optional[str] = ..., user: _Optional[str] = ..., password: _Optional[str] = ..., sslmode: _Optional[str] = ...) -> None: ...
