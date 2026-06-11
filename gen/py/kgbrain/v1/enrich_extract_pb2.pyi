from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class EnrichExtractOutputColumnType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_UNSPECIFIED: _ClassVar[EnrichExtractOutputColumnType]
    ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT: _ClassVar[EnrichExtractOutputColumnType]
    ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_BIGINT: _ClassVar[EnrichExtractOutputColumnType]

class EnrichExtractJobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ENRICH_EXTRACT_JOB_STATUS_UNSPECIFIED: _ClassVar[EnrichExtractJobStatus]
    ENRICH_EXTRACT_JOB_STATUS_PENDING: _ClassVar[EnrichExtractJobStatus]
    ENRICH_EXTRACT_JOB_STATUS_RUNNING: _ClassVar[EnrichExtractJobStatus]
    ENRICH_EXTRACT_JOB_STATUS_SUCCEEDED: _ClassVar[EnrichExtractJobStatus]
    ENRICH_EXTRACT_JOB_STATUS_PARTIAL: _ClassVar[EnrichExtractJobStatus]
    ENRICH_EXTRACT_JOB_STATUS_FAILED: _ClassVar[EnrichExtractJobStatus]
ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_UNSPECIFIED: EnrichExtractOutputColumnType
ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT: EnrichExtractOutputColumnType
ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_BIGINT: EnrichExtractOutputColumnType
ENRICH_EXTRACT_JOB_STATUS_UNSPECIFIED: EnrichExtractJobStatus
ENRICH_EXTRACT_JOB_STATUS_PENDING: EnrichExtractJobStatus
ENRICH_EXTRACT_JOB_STATUS_RUNNING: EnrichExtractJobStatus
ENRICH_EXTRACT_JOB_STATUS_SUCCEEDED: EnrichExtractJobStatus
ENRICH_EXTRACT_JOB_STATUS_PARTIAL: EnrichExtractJobStatus
ENRICH_EXTRACT_JOB_STATUS_FAILED: EnrichExtractJobStatus

class StartEnrichExtractRequest(_message.Message):
    __slots__ = ("llm_resource_id", "database_resource_id", "source_table", "output_table", "key_field", "output_schema", "target_example", "start_id", "end_id", "concurrency", "overwrite", "page_size", "max_retries", "source_json_field", "priority_field_hints", "auto_create_output_table")
    class PriorityFieldHintsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LLM_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    TARGET_EXAMPLE_FIELD_NUMBER: _ClassVar[int]
    START_ID_FIELD_NUMBER: _ClassVar[int]
    END_ID_FIELD_NUMBER: _ClassVar[int]
    CONCURRENCY_FIELD_NUMBER: _ClassVar[int]
    OVERWRITE_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    MAX_RETRIES_FIELD_NUMBER: _ClassVar[int]
    SOURCE_JSON_FIELD_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_HINTS_FIELD_NUMBER: _ClassVar[int]
    AUTO_CREATE_OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    llm_resource_id: str
    database_resource_id: str
    source_table: str
    output_table: str
    key_field: str
    output_schema: _containers.RepeatedCompositeFieldContainer[EnrichExtractOutputColumn]
    target_example: _containers.RepeatedCompositeFieldContainer[_struct_pb2.Struct]
    start_id: int
    end_id: int
    concurrency: int
    overwrite: bool
    page_size: int
    max_retries: int
    source_json_field: str
    priority_field_hints: _containers.ScalarMap[str, str]
    auto_create_output_table: bool
    def __init__(self, llm_resource_id: _Optional[str] = ..., database_resource_id: _Optional[str] = ..., source_table: _Optional[str] = ..., output_table: _Optional[str] = ..., key_field: _Optional[str] = ..., output_schema: _Optional[_Iterable[_Union[EnrichExtractOutputColumn, _Mapping]]] = ..., target_example: _Optional[_Iterable[_Union[_struct_pb2.Struct, _Mapping]]] = ..., start_id: _Optional[int] = ..., end_id: _Optional[int] = ..., concurrency: _Optional[int] = ..., overwrite: bool = ..., page_size: _Optional[int] = ..., max_retries: _Optional[int] = ..., source_json_field: _Optional[str] = ..., priority_field_hints: _Optional[_Mapping[str, str]] = ..., auto_create_output_table: bool = ...) -> None: ...

class EnrichExtractOutputColumn(_message.Message):
    __slots__ = ("name", "type")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    name: str
    type: EnrichExtractOutputColumnType
    def __init__(self, name: _Optional[str] = ..., type: _Optional[_Union[EnrichExtractOutputColumnType, str]] = ...) -> None: ...

class StartEnrichExtractResponse(_message.Message):
    __slots__ = ("job_id", "status")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    status: EnrichExtractJobStatus
    def __init__(self, job_id: _Optional[str] = ..., status: _Optional[_Union[EnrichExtractJobStatus, str]] = ...) -> None: ...

class GetEnrichExtractJobRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetEnrichExtractJobResponse(_message.Message):
    __slots__ = ("job_id", "llm_resource_id", "database_resource_id", "source_table", "output_table", "key_field", "source_json_field", "status", "error_message", "last_key", "processed_rows", "succeeded_rows", "failed_rows", "created_at_unix", "started_at_unix", "finished_at_unix", "output_schema", "auto_create_output_table")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    LLM_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_FIELD_NUMBER: _ClassVar[int]
    SOURCE_JSON_FIELD_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    LAST_KEY_FIELD_NUMBER: _ClassVar[int]
    PROCESSED_ROWS_FIELD_NUMBER: _ClassVar[int]
    SUCCEEDED_ROWS_FIELD_NUMBER: _ClassVar[int]
    FAILED_ROWS_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    AUTO_CREATE_OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    llm_resource_id: str
    database_resource_id: str
    source_table: str
    output_table: str
    key_field: str
    source_json_field: str
    status: EnrichExtractJobStatus
    error_message: str
    last_key: int
    processed_rows: int
    succeeded_rows: int
    failed_rows: int
    created_at_unix: int
    started_at_unix: int
    finished_at_unix: int
    output_schema: _containers.RepeatedCompositeFieldContainer[EnrichExtractOutputColumn]
    auto_create_output_table: bool
    def __init__(self, job_id: _Optional[str] = ..., llm_resource_id: _Optional[str] = ..., database_resource_id: _Optional[str] = ..., source_table: _Optional[str] = ..., output_table: _Optional[str] = ..., key_field: _Optional[str] = ..., source_json_field: _Optional[str] = ..., status: _Optional[_Union[EnrichExtractJobStatus, str]] = ..., error_message: _Optional[str] = ..., last_key: _Optional[int] = ..., processed_rows: _Optional[int] = ..., succeeded_rows: _Optional[int] = ..., failed_rows: _Optional[int] = ..., created_at_unix: _Optional[int] = ..., started_at_unix: _Optional[int] = ..., finished_at_unix: _Optional[int] = ..., output_schema: _Optional[_Iterable[_Union[EnrichExtractOutputColumn, _Mapping]]] = ..., auto_create_output_table: bool = ...) -> None: ...
