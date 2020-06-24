/*
Copyright (c) 2019 Dell EMC Corporation
All Rights Reserved
*/
package gounity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dell/gounity/util"

	"github.com/dell/gounity/api"
	"github.com/dell/gounity/types"
)

type filesystem struct {
	client *Client
}

const (
	FsNameMaxLength = 63
)

type AccessType string

const (
	ReadOnlyAccessType      = AccessType("READ_ONLY")
	ReadWriteAccessType     = AccessType("READ_WRITE")
	ReadOnlyRootAccessType  = AccessType("READ_ONLY_ROOT")
	ReadWriteRootAccessType = AccessType("READ_WRITE_ROOT")
)

type NFSShareDefaultAccess string

const (
	NoneDefaultAccess          = NFSShareDefaultAccess("0")
	ReadOnlyDefaultAccess      = NFSShareDefaultAccess("1")
	ReadWriteDefaultAccess     = NFSShareDefaultAccess("2")
	ReadOnlyRootDefaultAccess  = NFSShareDefaultAccess("3")
	ReadWriteRootDefaultAccess = NFSShareDefaultAccess("4")
)

var FilesystemNotFoundError = errors.New("Unable to find filesystem")

func NewFilesystem(client *Client) *filesystem {
	return &filesystem{client}
}

//FindFilesystemByName - Find the Filesystem by it's name. If the Filesystem is not found, an error will be returned.
func (f *filesystem) FindFilesystemByName(ctx context.Context, filesystemName string) (*types.Filesystem, error) {
	if len(filesystemName) == 0 {
		return nil, errors.New("Filesystem Name shouldn't be empty")
	}
	fileSystemResp := &types.Filesystem{}
	err := f.client.executeWithRetryAuthenticate(ctx, http.MethodGet, fmt.Sprintf(api.UnityApiGetResourceByNameWithFieldsUri, "filesystem", filesystemName, api.FileSystemDisplayFields), nil, fileSystemResp)
	if err != nil {
		return nil, FilesystemNotFoundError
	}
	return fileSystemResp, nil
}

//FindFilesystemById - Find the Filesystem by it's Id. If the Filesystem is not found, an error will be returned.
func (f *filesystem) FindFilesystemById(ctx context.Context, filesystemId string) (*types.Filesystem, error) {
	log := util.GetRunIdLogger(ctx)
	if len(filesystemId) == 0 {
		return nil, errors.New("Filesystem Id shouldn't be empty")
	}
	fileSystemResp := &types.Filesystem{}
	err := f.client.executeWithRetryAuthenticate(ctx, http.MethodGet, fmt.Sprintf(api.UnityApiGetResourceWithFieldsUri, "filesystem", filesystemId, api.FileSystemDisplayFields), nil, fileSystemResp)
	if err != nil {
		log.Debugf("Unable to find filesystem Id %s Error: %v", filesystemId, err)
		return nil, FilesystemNotFoundError
	}
	return fileSystemResp, nil
}

//CreateFilesystem - Create a new filesystem on the array
func (f *filesystem) CreateFilesystem(ctx context.Context, name, storagepool, description, nasServer string, size uint64, tieringPolicy, hostIOSize, supportedProtocol int, isThinEnabled, isDataReductionEnabled bool) (*types.Filesystem, error) {
	log := util.GetRunIdLogger(ctx)
	if name == "" {
		return nil, errors.New("filesystem name should not be empty.")
	}

	if len(name) > FsNameMaxLength {
		return nil, errors.New(fmt.Sprintf("filesystem name %s should not exceed %d characters.", name, FsNameMaxLength))
	}

	poolApi := NewStoragePool(f.client)
	pool, err := poolApi.FindStoragePoolById(ctx, storagepool)
	if pool == nil {
		return nil, errors.New(fmt.Sprintf("unable to get PoolID (%s) Error:%v", storagepool, err))
	}

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error trying to get Storage Pool (%s) Error:%v", storagepool, err))
	}

	storagePool := types.StoragePoolID{
		PoolId: storagepool,
	}

	fileEventSettings := types.FileEventSettings{
		IsCIFSEnabled: false, //Set to false to disable CIFS
		IsNFSEnabled:  true,  //Set to true to enable NFS alone
	}

	nas := types.NasServerID{
		NasServerID: nasServer,
	}

	fsParams := types.FsParameters{
		StoragePool:       &storagePool,
		Size:              size,
		SupportedProtocol: supportedProtocol,
		HostIOSize:        hostIOSize,
		NasServer:         &nas,
		FileEventSettings: fileEventSettings,
	}

	volApi := NewVolume(f.client)
	thinProvisioningLicenseInfoResp, err := volApi.isFeatureLicensed(ctx, ThinProvisioning)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to get license info for feature: %s", ThinProvisioning))
	}

	dataReductionLicenseInfoResp, err := volApi.isFeatureLicensed(ctx, DataReduction)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to get license info for feature: %s", DataReduction))
	}

	if thinProvisioningLicenseInfoResp.LicenseInfoContent.IsInstalled && thinProvisioningLicenseInfoResp.LicenseInfoContent.IsValid {
		fsParams.IsThinEnabled = strconv.FormatBool(isThinEnabled)
	} else if isThinEnabled == true {
		return nil, errors.New(fmt.Sprintf("Thin Provisioning is not supported on array and hence cannot create Filesystem."))
	}

	if dataReductionLicenseInfoResp.LicenseInfoContent.IsInstalled && dataReductionLicenseInfoResp.LicenseInfoContent.IsValid {
		fsParams.IsDataReductionEnabled = strconv.FormatBool(isDataReductionEnabled)
	} else if isDataReductionEnabled == true {
		return nil, errors.New(fmt.Sprintf("Data Reduction is not supported on array and hence cannot create Filesystem."))
	}

	if pool != nil && pool.StoragePoolContent.PoolFastVP.Status != 0 {
		log.Debug("FastVP is enabled")
		fastVPParameters := types.FastVPParameters{
			TieringPolicy: tieringPolicy,
		}
		fsParams.FastVPParameters = &fastVPParameters
	} else {
		log.Debug("FastVP is not enabled")
	}

	fileReqParam := types.FsCreateParam{
		Name:         name,
		Description:  description,
		FsParameters: &fsParams,
	}

	fileResp := &types.Filesystem{}
	err = f.client.executeWithRetryAuthenticate(ctx,
		http.MethodPost, fmt.Sprintf(api.UnityApiStorageResourceActionUri, "createFilesystem"), fileReqParam, fileResp)
	if err != nil {
		return nil, err
	}

	return fileResp, nil
}

//Delete Filesystem by its ID. If the Filesystem is not present on the array, an error will be returned.
func (f *filesystem) DeleteFilesystem(ctx context.Context, filesystemId string) error {
	log := util.GetRunIdLogger(ctx)
	if len(filesystemId) == 0 {
		return errors.New("Filesystem Id cannot be empty")
	}

	filesystemResp, err := f.FindFilesystemById(ctx, filesystemId)
	if err != nil {
		return FilesystemNotFoundError
	} else {
		resourceID := filesystemResp.FileContent.StorageResource.Id
		deleteErr := f.client.executeWithRetryAuthenticate(ctx, http.MethodDelete, fmt.Sprintf(api.UnityApiGetResourceUri, "storageResource", resourceID), nil, nil)
		if deleteErr != nil {
			return errors.New(fmt.Sprintf("Delete Filesystem %s Failed. Error: %v", filesystemId, deleteErr))
		}
		log.Debugf("Delete Filesystem %s Successful", filesystemId)
		return nil
	}
}

//Create NFSShare - Create NFS Share for a File system
func (f *filesystem) CreateNFSShare(ctx context.Context, name, path, filesystemId string, nfsShareDefaultAccess NFSShareDefaultAccess) (*types.Filesystem, error) {
	if len(filesystemId) == 0 {
		return nil, errors.New("Filesystem Id cannot be empty")
	}

	filesystemResp, err := f.FindFilesystemById(ctx, filesystemId)
	if err != nil {
		return nil, FilesystemNotFoundError
	}
	resourceID := filesystemResp.FileContent.StorageResource.Id

	nfsShareParam := types.NFSShareParameters{
		DefaultAccess: string(nfsShareDefaultAccess),
	}

	nfsShareCreateReqParam := types.NFSShareCreateParam{
		Name:               name,
		Path:               path,
		NFSShareParameters: &nfsShareParam,
	}

	nfsShares := []types.NFSShareCreateParam{nfsShareCreateReqParam}
	filesystemModifyParam := types.FsModifyParameters{
		NFSShares: &nfsShares,
	}

	err = f.client.executeWithRetryAuthenticate(ctx, http.MethodPost, fmt.Sprintf(api.UnityModifyFilesystemUri, resourceID), filesystemModifyParam, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Create NFS Share failed. Error: %v", err))
	}

	filesystemResp, err = f.FindFilesystemById(ctx, filesystemId)
	if err != nil {
		return nil, FilesystemNotFoundError
	}
	return filesystemResp, nil
}

//FindNFSShareByName - Find the NFS Share by it's name. If the NFS Share is not found, an error will be returned.
func (f *filesystem) FindNFSShareByName(ctx context.Context, nfsSharename string) (*types.NFSShare, error) {
	if len(nfsSharename) == 0 {
		return nil, errors.New("NFS Share Name shouldn't be empty")
	}
	nfsShareResp := &types.NFSShare{}
	err := f.client.executeWithRetryAuthenticate(ctx, http.MethodGet, fmt.Sprintf(api.UnityApiGetResourceByNameWithFieldsUri, "nfsShare", nfsSharename, api.NFSShareDisplayfields), nil, nfsShareResp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to find NFS Share. Error: %v", err))
	}
	return nfsShareResp, nil
}

//FindNFSShareById - Find the NFS Share by it's Id. If the NFS Share is not found, an error will be returned.
func (f *filesystem) FindNFSShareById(ctx context.Context, nfsShareId string) (*types.NFSShare, error) {
	if len(nfsShareId) == 0 {
		return nil, errors.New("NFS Share Id shouldn't be empty")
	}
	nfsShareResp := &types.NFSShare{}
	err := f.client.executeWithRetryAuthenticate(ctx, http.MethodGet, fmt.Sprintf(api.UnityApiGetResourceWithFieldsUri, "nfsShare", nfsShareId, api.NFSShareDisplayfields), nil, nfsShareResp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to find NFS Share: %s. Error: %v", nfsShareId, err))
	}
	return nfsShareResp, nil
}

//ModifyNFSShareHostAccess - Modify the host access on NFS Share
func (f *filesystem) ModifyNFSShareHostAccess(ctx context.Context, filesystemId, nfsShareId string, hostIds []string, accessType AccessType) error {
	log := util.GetRunIdLogger(ctx)
	if len(filesystemId) == 0 {
		return errors.New("Filesystem Id cannot be empty")
	}

	filesystemResp, err := f.FindFilesystemById(ctx, filesystemId)
	if err != nil {
		return FilesystemNotFoundError
	}
	resourceID := filesystemResp.FileContent.StorageResource.Id

	hostsIdsContent := []types.HostIdContent{}
	for _, hostId := range hostIds {
		hostIdContent := types.HostIdContent{
			ID: hostId,
		}
		hostsIdsContent = append(hostsIdsContent, hostIdContent)
	}

	nfsShareParameters := types.NFSShareParameters{}
	if accessType == ReadOnlyAccessType {
		nfsShareParameters.ReadOnlyHosts = &hostsIdsContent
	} else if accessType == ReadWriteAccessType {
		nfsShareParameters.ReadWriteHosts = &hostsIdsContent
	} else if accessType == ReadOnlyRootAccessType {
		nfsShareParameters.ReadOnlyRootAccessHosts = &hostsIdsContent
	} else {
		nfsShareParameters.RootAccessHosts = &hostsIdsContent
	}

	nfsShare := types.StorageResourceParam{
		ID: nfsShareId,
	}

	nfsShareModifyContent := types.NFSShareModifyContent{
		NFSShare:           &nfsShare,
		NFSShareParameters: &nfsShareParameters,
	}
	nfsSharesModifyContent := []types.NFSShareModifyContent{nfsShareModifyContent}

	nfsShareModifyReq := types.NFSShareModify{
		NFSSharesModifyContent: &nfsSharesModifyContent,
	}

	err = f.client.executeWithRetryAuthenticate(ctx, http.MethodPost, fmt.Sprintf(api.UnityModifyFilesystemUri, resourceID), nfsShareModifyReq, nil)
	if err != nil {
		return errors.New(fmt.Sprintf("Modify NFS Share failed. Error: %v", err))
	}
	log.Debugf("Modify NFS share: %s successful. Added host with access %s", nfsShareId, accessType)
	return nil
}

//DeleteNFSShare by its ID. If the NFSShare is not present on the array, an error will be returned.
func (f *filesystem) DeleteNFSShare(ctx context.Context, filesystemId, nfsShareId string) error {
	log := util.GetRunIdLogger(ctx)

	if len(filesystemId) == 0 {
		return errors.New("Filesystem Id cannot be empty")
	}
	filesystemResp, err := f.FindFilesystemById(ctx, filesystemId)
	if err != nil {
		return FilesystemNotFoundError
	}
	resourceID := filesystemResp.FileContent.StorageResource.Id

	if len(nfsShareId) == 0 {
		return errors.New("NFS Share Id cannot be empty")
	}
	_, err = f.FindNFSShareById(ctx, nfsShareId)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to find NFS Share. Error: %v", err))
	}

	nfsShare := types.StorageResourceParam{
		ID: nfsShareId,
	}

	nfsShareDeleteContent := types.NFSShareModifyContent{
		NFSShare: &nfsShare,
	}
	nfsSharesDeleteContent := []types.NFSShareModifyContent{nfsShareDeleteContent}

	nfsShareDeleteReq := types.NFSShareDelete{
		NFSSharesDeleteContent: &nfsSharesDeleteContent,
	}

	deleteErr := f.client.executeWithRetryAuthenticate(ctx, http.MethodPost, fmt.Sprintf(api.UnityModifyFilesystemUri, resourceID), nfsShareDeleteReq, nil)
	if deleteErr != nil {
		return errors.New(fmt.Sprintf("Delete NFS Share: %s Failed. Error: %v", nfsShareId, deleteErr))
	}
	log.Infof("Delete NFS Share: %s Successful", nfsShareId)
	return nil
}

//FindNASServerById - Find the NAS Server by it's Id. If the NAS Server is not found, an error will be returned.
func (f *filesystem) FindNASServerById(ctx context.Context, nasServerId string) (*types.NASServer, error) {
	if len(nasServerId) == 0 {
		return nil, errors.New("NAS Server Id shouldn't be empty")
	}
	nasServerResp := &types.NASServer{}
	err := f.client.executeWithRetryAuthenticate(ctx, http.MethodGet, fmt.Sprintf(api.UnityApiGetResourceWithFieldsUri, "nasServer", nasServerId, api.NasServerDisplayfields), nil, nasServerResp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to find NAS Server: %s. Error: %v", nasServerId, err))
	}
	return nasServerResp, nil
}
