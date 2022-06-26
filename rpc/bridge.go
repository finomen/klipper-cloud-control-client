package rpc

import (
	"bytes"
	"github.com/finomen/go-moonraker-api/api"
	"github.com/finomen/go-moonraker-api/jsonrpc"
	"io/ioutil"
	"klipper-cloud-control-client/config"
	"log"
	"net/http"
	"net/http/cookiejar"
	"path"
	"time"
)

const (
	timeout = time.Second * 15
)

type Bridge struct {
	printerConnection *jsonrpc.Client
	cloudConnection   *jsonrpc.Client
	jar               *cookiejar.Jar
}

func bind[Request interface{}, Response interface{}](method jsonrpc.Method[Request, Response], from *jsonrpc.Client, to *jsonrpc.Client) {
	method.Listen(func(req *Request) Response {
		var def Response
		//log.Println("->", method.Name)
		var reqValue Request
		if req != nil {
			reqValue = *req
		}
		res, err := method.Call(reqValue, timeout, to)
		//log.Println("<-", method.Name)
		if err != nil {
			log.Println("Call ", method.Name, " failed: ", err)
			//FIXME: return error!
			return def
		}
		if res == nil {
			//FIXME: should not happen
			return def
		}
		return *res
	}, from)
}

func cloudToPrinter[Request interface{}, Response interface{}](method jsonrpc.Method[Request, Response], bridge *Bridge) {
	bind(method, bridge.cloudConnection, bridge.printerConnection)
}

func printerToCloud[Request interface{}](method jsonrpc.Notify[Request], bridge *Bridge) {
	method.Listen(func(req *Request) {
		var reqValue Request
		if req != nil {
			reqValue = *req
		}
		method.Send(reqValue, bridge.cloudConnection)
	}, bridge.printerConnection)
}

func (b *Bridge) uploadFile(path string, id string) {
	client := http.Client{ // TODO: share client? make upload queue
		Jar: b.jar,
	}
	log.Println("Start upload ", path)
	file, err := http.Get(config.GetConfig().MoonrakerUrl + path)
	if err != nil {
		log.Println("Get file failed")
		return
	}

	data, err := ioutil.ReadAll(file.Body)

	if err != nil {
		log.Println("Get file failed")
		return
	}

	_, err = client.Post(config.GetConfig().GetHostname()+"/api/download?download-id="+id, file.Header.Get("Content-Type"), bytes.NewBuffer(data))

	if err != nil {
		log.Println("Get file failed")
		return
	}
}

func NewBridge(cloudRx chan []byte, cloudTx chan []byte, printerRx chan []byte, printerTx chan []byte, jar *cookiejar.Jar) *Bridge {
	bridge := &Bridge{
		printerConnection: jsonrpc.NewClient(printerRx, printerTx),
		cloudConnection:   jsonrpc.NewClient(cloudRx, cloudTx),
		jar:               jar,
	}
	cloudToPrinter(api.ServerConnectionIdentity, bridge)
	cloudToPrinter(api.GetWebsocketId, bridge)
	cloudToPrinter(api.PrinterInfo, bridge)
	cloudToPrinter(api.PrinterEmergencyStop, bridge)
	cloudToPrinter(api.PrinterRestart, bridge)
	cloudToPrinter(api.PrinterFirmwareRestart, bridge)
	cloudToPrinter(api.PrinterObjectsList, bridge)
	cloudToPrinter(api.PrinterObjectsQuery, bridge)
	cloudToPrinter(api.PrinterObjectsSubscribe, bridge)
	cloudToPrinter(api.PrinterQueryEndstopsStatus, bridge)
	cloudToPrinter(api.Serverinfo, bridge)
	cloudToPrinter(api.ServerConfig, bridge)
	cloudToPrinter(api.ServerTemperatureStore, bridge)
	cloudToPrinter(api.ServerGCodeStore, bridge)
	cloudToPrinter(api.ServerRestart, bridge)
	cloudToPrinter(api.PrinterGCodeScript, bridge)
	cloudToPrinter(api.PrinterGCodeHelp, bridge)
	cloudToPrinter(api.PrinterPrintStart, bridge)
	cloudToPrinter(api.PrinterPrintPause, bridge)
	cloudToPrinter(api.PrinterPrintResume, bridge)
	cloudToPrinter(api.PrinterPrintCancel, bridge)
	cloudToPrinter(api.MachineSystemInfo, bridge)
	cloudToPrinter(api.MachineShutdown, bridge)
	cloudToPrinter(api.MachineReboot, bridge)
	cloudToPrinter(api.MachineServicesRestart, bridge)
	cloudToPrinter(api.MachineServicesStop, bridge)
	cloudToPrinter(api.MachineServicesStart, bridge)
	cloudToPrinter(api.MachineProcStats, bridge)
	cloudToPrinter(api.ServerFilesList, bridge)
	cloudToPrinter(api.ServerFilesMetadata, bridge)
	cloudToPrinter(api.ServerFilesGetDirectory, bridge)
	cloudToPrinter(api.ServerFilesPostDirectory, bridge)
	cloudToPrinter(api.ServerFilesDeleteDirectory, bridge)
	cloudToPrinter(api.ServerFilesMove, bridge)
	cloudToPrinter(api.ServerFilesCopy, bridge)
	//TODO: CloudDownload
	//TODO: CloudUpload
	cloudToPrinter(api.ServerFilesDeleteFile, bridge)
	cloudToPrinter(api.ServerDatabaseList, bridge)
	cloudToPrinter(api.ServerDatabaseGetItem, bridge)
	cloudToPrinter(api.ServerDatabasePostItem, bridge)
	cloudToPrinter(api.ServerDatabaseDeleteItem, bridge)
	cloudToPrinter(api.ServerJobQueueStatus, bridge)
	cloudToPrinter(api.ServerJobQueuePostJob, bridge)
	cloudToPrinter(api.ServerJobQueueDeleteJob, bridge)
	cloudToPrinter(api.ServerJobQueuePause, bridge)
	cloudToPrinter(api.ServerJobQueueStart, bridge)
	cloudToPrinter(api.ServerAnnouncementsList, bridge)
	cloudToPrinter(api.ServerAnnouncementsUpdate, bridge)
	cloudToPrinter(api.ServerAnnouncementsDismiss, bridge)
	cloudToPrinter(api.ServerAnnouncementsFeeds, bridge)
	cloudToPrinter(api.ServerAnnouncementsPostFeed, bridge)
	cloudToPrinter(api.ServerAnnouncementsDeleteFeed, bridge)
	cloudToPrinter(api.MachineUpdateStatus, bridge)
	cloudToPrinter(api.MachineUpdateFull, bridge)
	cloudToPrinter(api.MachineUpdateMoonraker, bridge)
	cloudToPrinter(api.MachineUpdateKlipper, bridge)
	cloudToPrinter(api.MachineUpdateClient, bridge)
	cloudToPrinter(api.MachineUpdateSystem, bridge)
	cloudToPrinter(api.MachineUpdateRecover, bridge)
	cloudToPrinter(api.ServerHistoryList, bridge)
	cloudToPrinter(api.ServerHistoryTotals, bridge)
	cloudToPrinter(api.ServerHistoryResetTotals, bridge)
	cloudToPrinter(api.ServerHistoryGetJob, bridge)
	cloudToPrinter(api.ServerHistoryDeleteJob, bridge)

	printerToCloud(api.NotifyGCodeResponse, bridge)
	printerToCloud(api.NotifyStatusUpdate, bridge)
	printerToCloud(api.NotifyKlippyReady, bridge)
	printerToCloud(api.NotifyKlippyShutdown, bridge)
	printerToCloud(api.NotifyKlippyDisconnected, bridge)
	printerToCloud(api.NotifyFileListChanged, bridge)
	printerToCloud(api.NotifyUpdateResponse, bridge)
	printerToCloud(api.NotifyUpdateRefreshed, bridge)
	printerToCloud(api.NotifyCpuThrottled, bridge)
	printerToCloud(api.NotifyProcStatUpdate, bridge)
	printerToCloud(api.NotifyHistoryChanged, bridge)
	printerToCloud(api.NotifyUserCreated, bridge)
	printerToCloud(api.NotifyUserDeleted, bridge)
	printerToCloud(api.NotifyServiceStateChanged, bridge)
	printerToCloud(api.NotifyJobQueueChanged, bridge)

	api.CloudUpload.Listen(func(request *api.CloudUploadRequest) api.CloudUploadResponse {
		//TODO: wait for success
		go bridge.uploadFile(path.Join(request.Root, request.Path), request.DownloadId.String())
		return api.CloudUploadResponse{
			Status: 200, //TODO:
		}
	}, bridge.cloudConnection)

	return bridge
}
