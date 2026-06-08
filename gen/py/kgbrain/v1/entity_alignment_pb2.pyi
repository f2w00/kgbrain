from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class EntityAlignmentJobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ENTITY_ALIGNMENT_JOB_STATUS_UNSPECIFIED: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_PENDING: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_RUNNING: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_FAILED: _ClassVar[EntityAlignmentJobStatus]
ENTITY_ALIGNMENT_JOB_STATUS_UNSPECIFIED: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_PENDING: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_RUNNING: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_FAILED: EntityAlignmentJobStatus

class StartEntityAlignmentRequest(_message.Message):
    __slots__ = ("llm_resource_id", "database_resource_id", "source_table", "output_table", "reuse_mapping", "fields")
    LLM_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    REUSE_MAPPING_FIELD_NUMBER: _ClassVar[int]
    FIELDS_FIELD_NUMBER: _ClassVar[int]
    llm_resource_id: str
    database_resource_id: str
    source_table: str
    output_table: str
    reuse_mapping: bool
    fields: _containers.RepeatedCompositeFieldContainer[EntityAlignmentField]
    def __init__(self, llm_resource_id: _Optional[str] = ..., database_resource_id: _Optional[str] = ..., source_table: _Optional[str] = ..., output_table: _Optional[str] = ..., reuse_mapping: bool = ..., fields: _Optional[_Iterable[_Union[EntityAlignmentField, _Mapping]]] = ...) -> None: ...

class EntityAlignmentField(_message.Message):
    __slots__ = ("name", "targets", "batch_size")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    name: str
    targets: _containers.RepeatedScalarFieldContainer[str]
    batch_size: int
    def __init__(self, name: _Optional[str] = ..., targets: _Optional[_Iterable[str]] = ..., batch_size: _Optional[int] = ...) -> None: ...

class StartEntityAlignmentResponse(_message.Message):
    __slots__ = ("job_id", "status")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    status: EntityAlignmentJobStatus
    def __init__(self, job_id: _Optional[str] = ..., status: _Optional[_Union[EntityAlignmentJobStatus, str]] = ...) -> None: ...

class GetEntityAlignmentJobRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetEntityAlignmentJobResponse(_message.Message):
    __slots__ = ("job_id", "llm_resource_id", "database_resource_id", "source_table", "output_table", "status", "error_message", "created_at_unix", "started_at_unix", "finished_at_unix")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    LLM_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    llm_resource_id: str
    database_resource_id: str
    source_table: str
    output_table: str
    status: EntityAlignmentJobStatus
    error_message: str
    created_at_unix: int
    started_at_unix: int
    finished_at_unix: int
    def __init__(self, job_id: _Optional[str] = ..., llm_resource_id: _Optional[str] = ..., database_resource_id: _Optional[str] = ..., source_table: _Optional[str] = ..., output_table: _Optional[str] = ..., status: _Optional[_Union[EntityAlignmentJobStatus, str]] = ..., error_message: _Optional[str] = ..., created_at_unix: _Optional[int] = ..., started_at_unix: _Optional[int] = ..., finished_at_unix: _Optional[int] = ...) -> None: ...
