/*
Constants for utils
*/

package controller

const modelStorageVolumeName = "model-storage"
const modelStorageRoot = "/model-cache"
const pathSep = "/"
const ociPathToModelSep = "::"
const DECODE_ROLE = "decode"
const PREFILL_ROLE = "prefill"
const MODEL_ARTIFACT_URI_PVC = "pvc"
const MODEL_ARTIFACT_URI_HF = "hf"
const MODEL_ARTIFACT_URI_OCI = "oci"
const MODEL_ARTIFACT_URI_PVC_PREFIX = MODEL_ARTIFACT_URI_PVC + "://"
const MODEL_ARTIFACT_URI_HF_PREFIX = MODEL_ARTIFACT_URI_HF + "://"
const MODEL_ARTIFACT_URI_OCI_PREFIX = MODEL_ARTIFACT_URI_OCI + "://"
const ENV_HF_HOME = "HF_HOME"
const ENV_HF_TOKEN = "HF_TOKEN"

type URIType string

const (
	PVC        URIType = "pvc"
	HF         URIType = "hf"
	OCI        URIType = "oci"
	UnknownURI URIType = "unknown"
)
