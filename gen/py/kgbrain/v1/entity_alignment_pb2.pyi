from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class TargetCandidateStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TARGET_CANDIDATE_STATUS_UNSPECIFIED: _ClassVar[TargetCandidateStatus]
    TARGET_CANDIDATE_STATUS_PENDING: _ClassVar[TargetCandidateStatus]
    TARGET_CANDIDATE_STATUS_RESOLVED: _ClassVar[TargetCandidateStatus]

class TargetCandidateResolution(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TARGET_CANDIDATE_RESOLUTION_UNSPECIFIED: _ClassVar[TargetCandidateResolution]
    TARGET_CANDIDATE_RESOLUTION_ADD_AS_LABEL: _ClassVar[TargetCandidateResolution]
    TARGET_CANDIDATE_RESOLUTION_MAP_TO_EXISTING: _ClassVar[TargetCandidateResolution]
    TARGET_CANDIDATE_RESOLUTION_REJECT_AS_NULL: _ClassVar[TargetCandidateResolution]

class EntityAlignmentJobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ENTITY_ALIGNMENT_JOB_STATUS_UNSPECIFIED: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_PENDING: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_RUNNING: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED: _ClassVar[EntityAlignmentJobStatus]
    ENTITY_ALIGNMENT_JOB_STATUS_FAILED: _ClassVar[EntityAlignmentJobStatus]
TARGET_CANDIDATE_STATUS_UNSPECIFIED: TargetCandidateStatus
TARGET_CANDIDATE_STATUS_PENDING: TargetCandidateStatus
TARGET_CANDIDATE_STATUS_RESOLVED: TargetCandidateStatus
TARGET_CANDIDATE_RESOLUTION_UNSPECIFIED: TargetCandidateResolution
TARGET_CANDIDATE_RESOLUTION_ADD_AS_LABEL: TargetCandidateResolution
TARGET_CANDIDATE_RESOLUTION_MAP_TO_EXISTING: TargetCandidateResolution
TARGET_CANDIDATE_RESOLUTION_REJECT_AS_NULL: TargetCandidateResolution
ENTITY_ALIGNMENT_JOB_STATUS_UNSPECIFIED: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_PENDING: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_RUNNING: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED: EntityAlignmentJobStatus
ENTITY_ALIGNMENT_JOB_STATUS_FAILED: EntityAlignmentJobStatus

class StartEntityAlignmentRequest(_message.Message):
    __slots__ = ("llm_resource_id", "database_resource_id", "source_table", "output_table", "fields", "key_field", "start_id", "end_id", "only_waiting_target_review")
    LLM_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TABLE_FIELD_NUMBER: _ClassVar[int]
    FIELDS_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_FIELD_NUMBER: _ClassVar[int]
    START_ID_FIELD_NUMBER: _ClassVar[int]
    END_ID_FIELD_NUMBER: _ClassVar[int]
    ONLY_WAITING_TARGET_REVIEW_FIELD_NUMBER: _ClassVar[int]
    llm_resource_id: str
    database_resource_id: str
    source_table: str
    output_table: str
    fields: _containers.RepeatedCompositeFieldContainer[EntityAlignmentField]
    key_field: str
    start_id: int
    end_id: int
    only_waiting_target_review: bool
    def __init__(self, llm_resource_id: _Optional[str] = ..., database_resource_id: _Optional[str] = ..., source_table: _Optional[str] = ..., output_table: _Optional[str] = ..., fields: _Optional[_Iterable[_Union[EntityAlignmentField, _Mapping]]] = ..., key_field: _Optional[str] = ..., start_id: _Optional[int] = ..., end_id: _Optional[int] = ..., only_waiting_target_review: bool = ...) -> None: ...

class EntityAlignmentField(_message.Message):
    __slots__ = ("name", "target_set_id", "batch_size", "batch_concurrency")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    BATCH_CONCURRENCY_FIELD_NUMBER: _ClassVar[int]
    name: str
    target_set_id: str
    batch_size: int
    batch_concurrency: int
    def __init__(self, name: _Optional[str] = ..., target_set_id: _Optional[str] = ..., batch_size: _Optional[int] = ..., batch_concurrency: _Optional[int] = ...) -> None: ...

class ListAlignmentTargetsRequest(_message.Message):
    __slots__ = ("database_resource_id", "target_set_id")
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    database_resource_id: str
    target_set_id: str
    def __init__(self, database_resource_id: _Optional[str] = ..., target_set_id: _Optional[str] = ...) -> None: ...

class AlignmentTarget(_message.Message):
    __slots__ = ("target_set_id", "label", "description")
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    target_set_id: str
    label: str
    description: str
    def __init__(self, target_set_id: _Optional[str] = ..., label: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class ListAlignmentTargetsResponse(_message.Message):
    __slots__ = ("targets",)
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    targets: _containers.RepeatedCompositeFieldContainer[AlignmentTarget]
    def __init__(self, targets: _Optional[_Iterable[_Union[AlignmentTarget, _Mapping]]] = ...) -> None: ...

class UpsertAlignmentTargetsRequest(_message.Message):
    __slots__ = ("database_resource_id", "target_set_id", "targets")
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    database_resource_id: str
    target_set_id: str
    targets: _containers.RepeatedCompositeFieldContainer[AlignmentTarget]
    def __init__(self, database_resource_id: _Optional[str] = ..., target_set_id: _Optional[str] = ..., targets: _Optional[_Iterable[_Union[AlignmentTarget, _Mapping]]] = ...) -> None: ...

class UpsertAlignmentTargetsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeleteAlignmentTargetRequest(_message.Message):
    __slots__ = ("database_resource_id", "target_set_id", "label")
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    database_resource_id: str
    target_set_id: str
    label: str
    def __init__(self, database_resource_id: _Optional[str] = ..., target_set_id: _Optional[str] = ..., label: _Optional[str] = ...) -> None: ...

class DeleteAlignmentTargetResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class TargetCandidate(_message.Message):
    __slots__ = ("id", "target_set_id", "raw_value", "frequency", "status", "resolution", "resolved_label", "review_reason", "created_at_unix", "updated_at_unix")
    ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    RAW_VALUE_FIELD_NUMBER: _ClassVar[int]
    FREQUENCY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_LABEL_FIELD_NUMBER: _ClassVar[int]
    REVIEW_REASON_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    id: str
    target_set_id: str
    raw_value: str
    frequency: int
    status: TargetCandidateStatus
    resolution: TargetCandidateResolution
    resolved_label: str
    review_reason: str
    created_at_unix: int
    updated_at_unix: int
    def __init__(self, id: _Optional[str] = ..., target_set_id: _Optional[str] = ..., raw_value: _Optional[str] = ..., frequency: _Optional[int] = ..., status: _Optional[_Union[TargetCandidateStatus, str]] = ..., resolution: _Optional[_Union[TargetCandidateResolution, str]] = ..., resolved_label: _Optional[str] = ..., review_reason: _Optional[str] = ..., created_at_unix: _Optional[int] = ..., updated_at_unix: _Optional[int] = ...) -> None: ...

class ListTargetCandidatesRequest(_message.Message):
    __slots__ = ("database_resource_id", "target_set_id", "status")
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    database_resource_id: str
    target_set_id: str
    status: TargetCandidateStatus
    def __init__(self, database_resource_id: _Optional[str] = ..., target_set_id: _Optional[str] = ..., status: _Optional[_Union[TargetCandidateStatus, str]] = ...) -> None: ...

class ListTargetCandidatesResponse(_message.Message):
    __slots__ = ("candidates",)
    CANDIDATES_FIELD_NUMBER: _ClassVar[int]
    candidates: _containers.RepeatedCompositeFieldContainer[TargetCandidate]
    def __init__(self, candidates: _Optional[_Iterable[_Union[TargetCandidate, _Mapping]]] = ...) -> None: ...

class ReviewTargetCandidateAction(_message.Message):
    __slots__ = ("candidate_id", "resolution", "label", "review_reason")
    CANDIDATE_ID_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    REVIEW_REASON_FIELD_NUMBER: _ClassVar[int]
    candidate_id: str
    resolution: TargetCandidateResolution
    label: str
    review_reason: str
    def __init__(self, candidate_id: _Optional[str] = ..., resolution: _Optional[_Union[TargetCandidateResolution, str]] = ..., label: _Optional[str] = ..., review_reason: _Optional[str] = ...) -> None: ...

class ReviewTargetCandidatesRequest(_message.Message):
    __slots__ = ("database_resource_id", "target_set_id", "actions", "source_table")
    DATABASE_RESOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_SET_ID_FIELD_NUMBER: _ClassVar[int]
    ACTIONS_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TABLE_FIELD_NUMBER: _ClassVar[int]
    database_resource_id: str
    target_set_id: str
    actions: _containers.RepeatedCompositeFieldContainer[ReviewTargetCandidateAction]
    source_table: str
    def __init__(self, database_resource_id: _Optional[str] = ..., target_set_id: _Optional[str] = ..., actions: _Optional[_Iterable[_Union[ReviewTargetCandidateAction, _Mapping]]] = ..., source_table: _Optional[str] = ...) -> None: ...

class ReviewTargetCandidatesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

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
