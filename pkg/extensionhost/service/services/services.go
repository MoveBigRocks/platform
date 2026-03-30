package serviceapp

import internalserviceapp "github.com/movebigrocks/platform/internal/service/services"

type AttachmentServiceConfig = internalserviceapp.AttachmentServiceConfig
type CreateQueueParams = internalserviceapp.CreateQueueParams
type CreateCaseParams = internalserviceapp.CreateCaseParams
type QueueService = internalserviceapp.QueueService
type CaseService = internalserviceapp.CaseService
type CaseServiceOption = internalserviceapp.CaseServiceOption

var NewAttachmentService = internalserviceapp.NewAttachmentService
var NewQueueService = internalserviceapp.NewQueueService
var NewCaseService = internalserviceapp.NewCaseService
var WithQueueItemStore = internalserviceapp.WithQueueItemStore
var WithTransactionRunner = internalserviceapp.WithTransactionRunner
