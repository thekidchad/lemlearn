package video

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/mediaconvert"
	"github.com/aws/aws-sdk-go-v2/service/mediaconvert/types"
)

// Encoder lance le transcodage d'une source déposée.
type Encoder interface {
	// Start renvoie l'identifiant du travail lancé.
	Start(ctx context.Context, asset Asset, bucket string) (jobID string, err error)
}

// JobState est l'avancement d'un transcodage.
type JobState string

const (
	JobRunning  JobState = "running"
	JobComplete JobState = "complete"
	JobFailed   JobState = "failed"
)

// JobWatcher interroge l'avancement d'un travail.
//
// Séparé d'Encoder : un encodeur de test sait lancer sans savoir surveiller,
// et le service dégrade proprement quand l'implémentation ne suit pas.
type JobWatcher interface {
	// Status renvoie l'état et, si le travail est terminé, la durée réelle
	// de la vidéo mesurée à l'encodage — plus fiable que celle déclarée par
	// le navigateur, et c'est elle qui borne le calcul d'assiduité.
	Status(ctx context.Context, jobID string) (JobState, int64, error)
}

// MediaConvert transcode en HLS multi-débit.
type MediaConvert struct {
	api      *mediaconvert.Client
	roleARN  string
	queueARN string
}

// NewMediaConvert construit l'encodeur.
//
// roleARN est le rôle que MediaConvert endosse pour lire la source et écrire
// les rendus : c'est lui qui porte les droits, pas notre Lambda.
func NewMediaConvert(ctx context.Context, endpoint, roleARN, queueARN string) (*MediaConvert, error) {
	if roleARN == "" {
		return nil, fmt.Errorf("video: rôle MediaConvert non configuré")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("video: configuration aws: %w", err)
	}

	// MediaConvert expose historiquement un point d'entrée propre à chaque
	// compte. Plutôt que d'en faire une variable à renseigner à la main — donc
	// à oublier — on le demande au service quand il n'est pas fourni, et on
	// retombe sur le point d'entrée régional si l'appel échoue.
	if endpoint == "" {
		probe := mediaconvert.NewFromConfig(cfg)
		if out, err := probe.DescribeEndpoints(ctx, &mediaconvert.DescribeEndpointsInput{}); err == nil {
			if len(out.Endpoints) > 0 && out.Endpoints[0].Url != nil {
				endpoint = *out.Endpoints[0].Url
			}
		}
	}

	api := mediaconvert.NewFromConfig(cfg, func(o *mediaconvert.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &MediaConvert{api: api, roleARN: roleARN, queueARN: queueARN}, nil
}

// Rendus produits. Trois débits suffisent à couvrir la connexion d'un
// apprenant en formation : un partage de connexion en mobilité, un ADSL de
// province, une fibre. En proposer six coûterait le double d'encodage pour un
// confort que personne ne remarque.
var renditions = []struct {
	name    string
	height  int32
	bitrate int32
}{
	{"360p", 360, 800_000},
	{"540p", 540, 1_600_000},
	{"720p", 720, 3_000_000},
}

// Start lance le transcodage.
func (m *MediaConvert) Start(ctx context.Context, asset Asset, bucket string) (string, error) {
	source := fmt.Sprintf("s3://%s/%s", bucket, asset.SourceKey)
	destination := fmt.Sprintf("s3://%s/hls/%s/", bucket, asset.ID)

	outputs := make([]types.Output, 0, len(renditions))
	for _, rendition := range renditions {
		outputs = append(outputs, types.Output{
			NameModifier: aws.String("-" + rendition.name),
			ContainerSettings: &types.ContainerSettings{
				Container: types.ContainerTypeM3u8,
			},
			VideoDescription: &types.VideoDescription{
				Height: aws.Int32(rendition.height),
				CodecSettings: &types.VideoCodecSettings{
					Codec: types.VideoCodecH264,
					H264Settings: &types.H264Settings{
						RateControlMode: types.H264RateControlModeQvbr,
						MaxBitrate:      aws.Int32(rendition.bitrate),
						// Segments de six secondes alignés sur les
						// images-clés : le lecteur peut alors changer de débit
						// sans redemander la position, et la reprise de lecture
						// tombe juste.
						GopSize:      aws.Float64(2),
						GopSizeUnits: types.H264GopSizeUnitsSeconds,
					},
				},
			},
			AudioDescriptions: []types.AudioDescription{{
				CodecSettings: &types.AudioCodecSettings{
					Codec: types.AudioCodecAac,
					AacSettings: &types.AacSettings{
						Bitrate:    aws.Int32(96_000),
						CodingMode: types.AacCodingModeCodingMode20,
						SampleRate: aws.Int32(48_000),
					},
				},
			}},
		})
	}

	input := &mediaconvert.CreateJobInput{
		Role: aws.String(m.roleARN),
		Settings: &types.JobSettings{
			Inputs: []types.Input{{
				FileInput: aws.String(source),
				AudioSelectors: map[string]types.AudioSelector{
					"Audio Selector 1": {DefaultSelection: types.AudioDefaultSelectionDefault},
				},
			}},
			OutputGroups: []types.OutputGroup{{
				Name: aws.String("HLS"),
				OutputGroupSettings: &types.OutputGroupSettings{
					Type: types.OutputGroupTypeHlsGroupSettings,
					HlsGroupSettings: &types.HlsGroupSettings{
						Destination:      aws.String(destination),
						SegmentLength:    aws.Int32(6),
						MinSegmentLength: aws.Int32(0),
					},
				},
				Outputs: outputs,
			}},
		},
		// L'identifiant de l'asset voyage avec le travail : la notification de
		// fin arrive par EventBridge sans contexte, et sans cette étiquette il
		// faudrait deviner à quel module elle se rapporte.
		UserMetadata: map[string]string{"assetId": asset.ID, "orgId": asset.OrgID},
	}
	if m.queueARN != "" {
		input.Queue = aws.String(m.queueARN)
	}

	job, err := m.api.CreateJob(ctx, input)
	if err != nil {
		return "", fmt.Errorf("video: création du travail de transcodage: %w", err)
	}
	if job.Job == nil || job.Job.Id == nil {
		return "", fmt.Errorf("video: travail créé sans identifiant")
	}
	return *job.Job.Id, nil
}

// MasterKeyFor est la clé du manifeste principal produit par le transcodage.
func MasterKeyFor(assetID string) string {
	return fmt.Sprintf("hls/%s/%s.m3u8", assetID, assetID)
}

// AssetIDFromJobMetadata retrouve l'asset concerné par une notification.
func AssetIDFromJobMetadata(metadata map[string]string) string {
	return strings.TrimSpace(metadata["assetId"])
}

// Status interroge MediaConvert sur l'avancement d'un travail.
func (m *MediaConvert) Status(ctx context.Context, jobID string) (JobState, int64, error) {
	out, err := m.api.GetJob(ctx, &mediaconvert.GetJobInput{Id: aws.String(jobID)})
	if err != nil {
		return "", 0, fmt.Errorf("video: état du travail %s: %w", jobID, err)
	}
	if out.Job == nil {
		return "", 0, fmt.Errorf("video: travail %s introuvable", jobID)
	}

	switch out.Job.Status {
	case types.JobStatusComplete:
		var durationMs int64
		// La durée mesurée à l'encodage prime sur celle déclarée par le
		// navigateur : c'est elle qui sert de dénominateur à l'assiduité.
		if out.Job.OutputGroupDetails != nil {
			for _, group := range out.Job.OutputGroupDetails {
				for _, detail := range group.OutputDetails {
					if detail.DurationInMs != nil && int64(*detail.DurationInMs) > durationMs {
						durationMs = int64(*detail.DurationInMs)
					}
				}
			}
		}
		return JobComplete, durationMs, nil
	case types.JobStatusError, types.JobStatusCanceled:
		return JobFailed, 0, nil
	default:
		return JobRunning, 0, nil
	}
}
