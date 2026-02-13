package storage

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tonyellard/kay-vee/internal/model"
)

var secureKey = []byte("kay-vee-local-key")

type ParameterRecord struct {
	Name           string
	Type           string
	CurrentVersion int64
	Versions       map[int64]ParameterVersion
	Labels         map[string]int64
	LastModifiedAt time.Time
}

type ParameterVersion struct {
	Version   int64
	Value     string
	Tier      string
	CreatedAt time.Time
}

type SecretRecord struct {
	Name          string
	ARN           string
	Description   string
	DeletedAt     *time.Time
	Versions      map[string]SecretVersion
	StageToID     map[string]string
	VersionStages map[string]map[string]struct{}
	CreatedAt     time.Time
	LastChangedAt time.Time
}

type SecretVersion struct {
	VersionID    string
	SecretString *string
	SecretBinary string
	CreatedAt    time.Time
}

type Store struct {
	mu          sync.RWMutex
	parameters  map[string]*ParameterRecord
	secrets     map[string]*SecretRecord
	secretByARN map[string]string
	accountID   string
	region      string
}

func NewStore(region, accountID string) *Store {
	if region == "" {
		region = "us-east-1"
	}
	if accountID == "" {
		accountID = "000000000000"
	}
	return &Store{
		parameters:  make(map[string]*ParameterRecord),
		secrets:     make(map[string]*SecretRecord),
		secretByARN: make(map[string]string),
		accountID:   accountID,
		region:      region,
	}
}

func (s *Store) PutParameter(req model.PutParameterRequest) (model.PutParameterResponse, error) {
	if req.Name == "" {
		return model.PutParameterResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.Type == "" {
		req.Type = "String"
	}
	if req.Tier == "" {
		req.Tier = "Standard"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.parameters[req.Name]
	if !exists {
		record = &ParameterRecord{
			Name:     req.Name,
			Type:     req.Type,
			Versions: make(map[int64]ParameterVersion),
			Labels:   make(map[string]int64),
		}
		s.parameters[req.Name] = record
	} else if !req.Overwrite {
		return model.PutParameterResponse{}, fmt.Errorf("ParameterAlreadyExists: %s", req.Name)
	}

	record.Type = req.Type
	record.CurrentVersion++
	now := time.Now().UTC()
	value := req.Value
	if req.Type == "SecureString" {
		value = encryptValue(req.Value)
	}
	record.Versions[record.CurrentVersion] = ParameterVersion{
		Version:   record.CurrentVersion,
		Value:     value,
		Tier:      req.Tier,
		CreatedAt: now,
	}
	record.LastModifiedAt = now

	return model.PutParameterResponse{Version: record.CurrentVersion, Tier: req.Tier}, nil
}

func (s *Store) GetParameter(name string, withDecryption bool) (model.Parameter, error) {
	baseName, selector := parseSelector(name)

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.parameters[baseName]
	if !ok {
		return model.Parameter{}, fmt.Errorf("ParameterNotFound: %s", baseName)
	}

	version := record.CurrentVersion
	if selector != "" {
		if parsed, err := strconv.ParseInt(selector, 10, 64); err == nil {
			version = parsed
		} else {
			labeledVersion, labelOK := record.Labels[selector]
			if !labelOK {
				return model.Parameter{}, fmt.Errorf("ParameterNotFound: %s:%s", baseName, selector)
			}
			version = labeledVersion
		}
	}

	v, exists := record.Versions[version]
	if !exists {
		return model.Parameter{}, fmt.Errorf("ParameterNotFound: %s:%d", baseName, version)
	}

	value := v.Value
	if record.Type == "SecureString" {
		if withDecryption {
			value = decryptValue(v.Value)
		} else {
			value = "ENCRYPTED"
		}
	}

	return model.Parameter{
		Name:             record.Name,
		Type:             record.Type,
		Value:            value,
		Version:          version,
		ARN:              fmt.Sprintf("arn:aws:ssm:%s:%s:parameter%s", s.region, s.accountID, record.Name),
		LastModifiedDate: record.LastModifiedAt,
	}, nil
}

func (s *Store) GetParameters(names []string, withDecryption bool) ([]model.Parameter, []string) {
	found := make([]model.Parameter, 0, len(names))
	invalid := make([]string, 0)
	for _, name := range names {
		param, err := s.GetParameter(name, withDecryption)
		if err != nil {
			invalid = append(invalid, name)
			continue
		}
		found = append(found, param)
	}
	return found, invalid
}

func (s *Store) GetParametersByPath(path string, recursive, withDecryption bool) ([]model.Parameter, error) {
	if path == "" {
		return nil, fmt.Errorf("ValidationException: Path is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	normalized := strings.TrimSuffix(path, "/")
	if normalized == "" {
		normalized = "/"
	}

	names := make([]string, 0)
	for name := range s.parameters {
		if !matchesPath(name, normalized, recursive) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]model.Parameter, 0, len(names))
	for _, name := range names {
		record := s.parameters[name]
		version := record.CurrentVersion
		v := record.Versions[version]

		value := v.Value
		if record.Type == "SecureString" {
			if withDecryption {
				value = decryptValue(v.Value)
			} else {
				value = "ENCRYPTED"
			}
		}

		results = append(results, model.Parameter{
			Name:             record.Name,
			Type:             record.Type,
			Value:            value,
			Version:          version,
			ARN:              fmt.Sprintf("arn:aws:ssm:%s:%s:parameter%s", s.region, s.accountID, record.Name),
			LastModifiedDate: record.LastModifiedAt,
		})
	}

	return results, nil
}

func (s *Store) DescribeParameters() []model.ParameterMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.parameters))
	for name := range s.parameters {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]model.ParameterMetadata, 0, len(names))
	for _, name := range names {
		record := s.parameters[name]
		items = append(items, model.ParameterMetadata{
			Name:             record.Name,
			Type:             record.Type,
			Version:          record.CurrentVersion,
			ARN:              fmt.Sprintf("arn:aws:ssm:%s:%s:parameter%s", s.region, s.accountID, record.Name),
			LastModifiedDate: record.LastModifiedAt,
		})
	}

	return items
}

func (s *Store) GetParameterHistory(name string, withDecryption bool) ([]model.Parameter, error) {
	if name == "" {
		return nil, fmt.Errorf("ValidationException: Name is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.parameters[name]
	if !ok {
		return nil, fmt.Errorf("ParameterNotFound: %s", name)
	}

	versions := make([]int64, 0, len(record.Versions))
	for version := range record.Versions {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	history := make([]model.Parameter, 0, len(versions))
	for _, version := range versions {
		v := record.Versions[version]
		value := v.Value
		if record.Type == "SecureString" {
			if withDecryption {
				value = decryptValue(v.Value)
			} else {
				value = "ENCRYPTED"
			}
		}

		history = append(history, model.Parameter{
			Name:             record.Name,
			Type:             record.Type,
			Value:            value,
			Version:          version,
			ARN:              fmt.Sprintf("arn:aws:ssm:%s:%s:parameter%s", s.region, s.accountID, record.Name),
			LastModifiedDate: v.CreatedAt,
		})
	}

	return history, nil
}

func (s *Store) DeleteParameter(name string) error {
	if name == "" {
		return fmt.Errorf("ValidationException: Name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.parameters[name]; !exists {
		return fmt.Errorf("ParameterNotFound: %s", name)
	}
	delete(s.parameters, name)
	return nil
}

func (s *Store) DeleteParameters(names []string) (deleted []string, invalid []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted = make([]string, 0, len(names))
	invalid = make([]string, 0)
	for _, name := range names {
		if _, exists := s.parameters[name]; !exists {
			invalid = append(invalid, name)
			continue
		}
		delete(s.parameters, name)
		deleted = append(deleted, name)
	}
	return deleted, invalid
}

func (s *Store) LabelParameterVersion(req model.LabelParameterVersionRequest) (model.LabelParameterVersionResponse, error) {
	if req.Name == "" {
		return model.LabelParameterVersionResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.ParameterVersion <= 0 {
		return model.LabelParameterVersionResponse{}, fmt.Errorf("ValidationException: ParameterVersion must be > 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.parameters[req.Name]
	if !exists {
		return model.LabelParameterVersionResponse{}, fmt.Errorf("ParameterNotFound: %s", req.Name)
	}
	if _, versionExists := record.Versions[req.ParameterVersion]; !versionExists {
		return model.LabelParameterVersionResponse{}, fmt.Errorf("ParameterNotFound: %s:%d", req.Name, req.ParameterVersion)
	}

	invalid := make([]string, 0)
	for _, label := range req.Labels {
		if strings.TrimSpace(label) == "" {
			invalid = append(invalid, label)
			continue
		}
		record.Labels[label] = req.ParameterVersion
	}
	record.LastModifiedAt = time.Now().UTC()

	return model.LabelParameterVersionResponse{InvalidLabels: invalid, ParameterVersion: req.ParameterVersion}, nil
}

func (s *Store) CreateSecret(req model.CreateSecretRequest) (model.CreateSecretResponse, error) {
	if req.Name == "" {
		return model.CreateSecretResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.SecretString == nil && req.SecretBinary == "" {
		return model.CreateSecretResponse{}, fmt.Errorf("ValidationException: SecretString or SecretBinary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.secrets[req.Name]; exists {
		return model.CreateSecretResponse{}, fmt.Errorf("ResourceExistsException: %s", req.Name)
	}

	arn := fmt.Sprintf("arn:aws:secretsmanager:%s:%s:secret:%s-%s", s.region, s.accountID, req.Name, randomSuffix(6))
	versionID := randomVersionID()
	now := time.Now().UTC()

	record := &SecretRecord{
		Name:          req.Name,
		ARN:           arn,
		Description:   req.Description,
		Versions:      make(map[string]SecretVersion),
		StageToID:     make(map[string]string),
		VersionStages: make(map[string]map[string]struct{}),
		CreatedAt:     now,
		LastChangedAt: now,
	}
	record.Versions[versionID] = SecretVersion{
		VersionID:    versionID,
		SecretString: req.SecretString,
		SecretBinary: req.SecretBinary,
		CreatedAt:    now,
	}
	record.StageToID["AWSCURRENT"] = versionID
	record.VersionStages[versionID] = map[string]struct{}{"AWSCURRENT": {}}

	s.secrets[req.Name] = record
	s.secretByARN[arn] = req.Name

	return model.CreateSecretResponse{ARN: arn, Name: req.Name, VersionID: versionID}, nil
}

func (s *Store) PutSecretValue(req model.PutSecretValueRequest) (model.CreateSecretResponse, error) {
	if req.SecretID == "" {
		return model.CreateSecretResponse{}, fmt.Errorf("ValidationException: SecretId is required")
	}
	if req.SecretString == nil && req.SecretBinary == "" {
		return model.CreateSecretResponse{}, fmt.Errorf("ValidationException: SecretString or SecretBinary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.resolveSecretLocked(req.SecretID)
	if err != nil {
		return model.CreateSecretResponse{}, err
	}

	versionID := randomVersionID()
	now := time.Now().UTC()
	record.Versions[versionID] = SecretVersion{
		VersionID:    versionID,
		SecretString: req.SecretString,
		SecretBinary: req.SecretBinary,
		CreatedAt:    now,
	}
	record.VersionStages[versionID] = map[string]struct{}{}

	current := record.StageToID["AWSCURRENT"]
	if current != "" {
		delete(record.VersionStages[current], "AWSCURRENT")
		record.VersionStages[current]["AWSPREVIOUS"] = struct{}{}
		record.StageToID["AWSPREVIOUS"] = current
	}

	record.VersionStages[versionID]["AWSCURRENT"] = struct{}{}
	record.StageToID["AWSCURRENT"] = versionID
	record.LastChangedAt = now

	return model.CreateSecretResponse{ARN: record.ARN, Name: record.Name, VersionID: versionID}, nil
}

func (s *Store) UpdateSecret(req model.UpdateSecretRequest) (model.CreateSecretResponse, error) {
	if req.SecretID == "" {
		return model.CreateSecretResponse{}, fmt.Errorf("ValidationException: SecretId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.resolveSecretLocked(req.SecretID)
	if err != nil {
		return model.CreateSecretResponse{}, err
	}

	if req.Description != "" {
		record.Description = req.Description
	}
	record.LastChangedAt = time.Now().UTC()

	if req.SecretString == nil && req.SecretBinary == "" {
		return model.CreateSecretResponse{ARN: record.ARN, Name: record.Name, VersionID: record.StageToID["AWSCURRENT"]}, nil
	}

	versionID := randomVersionID()
	now := time.Now().UTC()
	record.Versions[versionID] = SecretVersion{
		VersionID:    versionID,
		SecretString: req.SecretString,
		SecretBinary: req.SecretBinary,
		CreatedAt:    now,
	}
	record.VersionStages[versionID] = map[string]struct{}{}

	current := record.StageToID["AWSCURRENT"]
	if current != "" {
		delete(record.VersionStages[current], "AWSCURRENT")
		record.VersionStages[current]["AWSPREVIOUS"] = struct{}{}
		record.StageToID["AWSPREVIOUS"] = current
	}
	record.VersionStages[versionID]["AWSCURRENT"] = struct{}{}
	record.StageToID["AWSCURRENT"] = versionID
	record.LastChangedAt = now

	return model.CreateSecretResponse{ARN: record.ARN, Name: record.Name, VersionID: versionID}, nil
}

func (s *Store) GetSecretValue(req model.GetSecretValueRequest) (model.SecretValueResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, err := s.resolveSecretReadLocked(req.SecretID)
	if err != nil {
		return model.SecretValueResponse{}, err
	}

	var versionID string
	switch {
	case req.VersionID != "":
		versionID = req.VersionID
	case req.VersionStage != "":
		versionID = record.StageToID[req.VersionStage]
	default:
		versionID = record.StageToID["AWSCURRENT"]
	}
	if versionID == "" {
		return model.SecretValueResponse{}, fmt.Errorf("ResourceNotFoundException: version not found")
	}

	v, ok := record.Versions[versionID]
	if !ok {
		return model.SecretValueResponse{}, fmt.Errorf("ResourceNotFoundException: version not found")
	}

	stages := make([]string, 0)
	for stage := range record.VersionStages[versionID] {
		stages = append(stages, stage)
	}

	return model.SecretValueResponse{
		ARN:          record.ARN,
		Name:         record.Name,
		VersionID:    versionID,
		SecretString: v.SecretString,
		SecretBinary: v.SecretBinary,
		VersionStage: stages,
		CreatedDate:  v.CreatedAt,
	}, nil
}

func (s *Store) DescribeSecret(secretID string) (model.DescribeSecretResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, err := s.resolveSecretReadLocked(secretID)
	if err != nil {
		return model.DescribeSecretResponse{}, err
	}

	versions := make([]model.SecretVersionStages, 0, len(record.VersionStages))
	for versionID, stageSet := range record.VersionStages {
		stages := make([]string, 0, len(stageSet))
		for stage := range stageSet {
			stages = append(stages, stage)
		}
		sort.Strings(stages)
		versions = append(versions, model.SecretVersionStages{VersionID: versionID, Stages: stages})
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].VersionID < versions[j].VersionID })

	return model.DescribeSecretResponse{
		ARN:                record.ARN,
		Name:               record.Name,
		Description:        record.Description,
		CreatedDate:        record.CreatedAt,
		LastChangedDate:    record.LastChangedAt,
		DeletedDate:        record.DeletedAt,
		VersionIDsToStages: versions,
	}, nil
}

func (s *Store) ListSecrets() model.ListSecretsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]model.SecretListEntry, 0, len(names))
	for _, name := range names {
		record := s.secrets[name]
		items = append(items, model.SecretListEntry{
			ARN:             record.ARN,
			Name:            record.Name,
			Description:     record.Description,
			CreatedDate:     record.CreatedAt,
			LastChangedDate: record.LastChangedAt,
			DeletedDate:     record.DeletedAt,
		})
	}

	return model.ListSecretsResponse{SecretList: items}
}

func (s *Store) DeleteSecret(req model.DeleteSecretRequest) (model.DeleteSecretResponse, error) {
	if req.SecretID == "" {
		return model.DeleteSecretResponse{}, fmt.Errorf("ValidationException: SecretId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.resolveSecretAnyLocked(req.SecretID)
	if err != nil {
		return model.DeleteSecretResponse{}, err
	}
	if record.DeletedAt != nil {
		return model.DeleteSecretResponse{}, fmt.Errorf("InvalidRequestException: secret is already scheduled for deletion")
	}

	deletionDate := time.Now().UTC()
	record.DeletedAt = &deletionDate
	record.LastChangedAt = deletionDate

	return model.DeleteSecretResponse{ARN: record.ARN, Name: record.Name, DeletionDate: deletionDate}, nil
}

func (s *Store) RestoreSecret(secretID string) (model.RestoreSecretResponse, error) {
	if secretID == "" {
		return model.RestoreSecretResponse{}, fmt.Errorf("ValidationException: SecretId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.resolveSecretAnyLocked(secretID)
	if err != nil {
		return model.RestoreSecretResponse{}, err
	}
	if record.DeletedAt == nil {
		return model.RestoreSecretResponse{}, fmt.Errorf("InvalidRequestException: secret is not deleted")
	}

	record.DeletedAt = nil
	record.LastChangedAt = time.Now().UTC()
	return model.RestoreSecretResponse{ARN: record.ARN, Name: record.Name}, nil
}

func (s *Store) UpdateSecretVersionStage(req model.UpdateSecretVersionStageRequest) (model.UpdateSecretVersionStageResponse, error) {
	if req.SecretID == "" {
		return model.UpdateSecretVersionStageResponse{}, fmt.Errorf("ValidationException: SecretId is required")
	}
	if req.VersionStage == "" {
		return model.UpdateSecretVersionStageResponse{}, fmt.Errorf("ValidationException: VersionStage is required")
	}
	if req.MoveToVersionID == "" {
		return model.UpdateSecretVersionStageResponse{}, fmt.Errorf("ValidationException: MoveToVersionId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.resolveSecretLocked(req.SecretID)
	if err != nil {
		return model.UpdateSecretVersionStageResponse{}, err
	}
	if _, exists := record.Versions[req.MoveToVersionID]; !exists {
		return model.UpdateSecretVersionStageResponse{}, fmt.Errorf("ResourceNotFoundException: version not found")
	}

	currentHolder := record.StageToID[req.VersionStage]
	if req.RemoveFromVersionID != "" && currentHolder != req.RemoveFromVersionID {
		return model.UpdateSecretVersionStageResponse{}, fmt.Errorf("InvalidRequestException: RemoveFromVersionId does not match current stage holder")
	}

	if currentHolder != "" && currentHolder != req.MoveToVersionID {
		delete(record.VersionStages[currentHolder], req.VersionStage)
	}

	if _, exists := record.VersionStages[req.MoveToVersionID]; !exists {
		record.VersionStages[req.MoveToVersionID] = map[string]struct{}{}
	}
	record.VersionStages[req.MoveToVersionID][req.VersionStage] = struct{}{}
	record.StageToID[req.VersionStage] = req.MoveToVersionID

	if req.VersionStage == "AWSCURRENT" && currentHolder != "" && currentHolder != req.MoveToVersionID {
		previousHolder := record.StageToID["AWSPREVIOUS"]
		if previousHolder != "" {
			delete(record.VersionStages[previousHolder], "AWSPREVIOUS")
		}
		record.VersionStages[currentHolder]["AWSPREVIOUS"] = struct{}{}
		record.StageToID["AWSPREVIOUS"] = currentHolder
	}

	record.LastChangedAt = time.Now().UTC()
	return model.UpdateSecretVersionStageResponse{ARN: record.ARN, Name: record.Name}, nil
}

func (s *Store) Summary() model.AdminSummaryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	active := 0
	deleted := 0
	for _, secret := range s.secrets {
		if secret.DeletedAt == nil {
			active++
		} else {
			deleted++
		}
	}

	return model.AdminSummaryResponse{
		Parameters:     len(s.parameters),
		SecretsTotal:   len(s.secrets),
		SecretsActive:  active,
		SecretsDeleted: deleted,
	}
}

func (s *Store) resolveSecretLocked(secretID string) (*SecretRecord, error) {
	if name, ok := s.secretByARN[secretID]; ok {
		rec, exists := s.secrets[name]
		if !exists {
			return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
		}
		if rec.DeletedAt != nil {
			return nil, fmt.Errorf("InvalidRequestException: secret is scheduled for deletion")
		}
		return rec, nil
	}
	rec, exists := s.secrets[secretID]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
	}
	if rec.DeletedAt != nil {
		return nil, fmt.Errorf("InvalidRequestException: secret is scheduled for deletion")
	}
	return rec, nil
}

func (s *Store) resolveSecretAnyLocked(secretID string) (*SecretRecord, error) {
	if name, ok := s.secretByARN[secretID]; ok {
		rec, exists := s.secrets[name]
		if !exists {
			return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
		}
		return rec, nil
	}
	rec, exists := s.secrets[secretID]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
	}
	return rec, nil
}

func (s *Store) resolveSecretReadLocked(secretID string) (*SecretRecord, error) {
	if secretID == "" {
		return nil, fmt.Errorf("ValidationException: SecretId is required")
	}
	if name, ok := s.secretByARN[secretID]; ok {
		rec, exists := s.secrets[name]
		if !exists {
			return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
		}
		if rec.DeletedAt != nil {
			return nil, fmt.Errorf("InvalidRequestException: secret is scheduled for deletion")
		}
		return rec, nil
	}
	rec, exists := s.secrets[secretID]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: %s", secretID)
	}
	if rec.DeletedAt != nil {
		return nil, fmt.Errorf("InvalidRequestException: secret is scheduled for deletion")
	}
	return rec, nil
}

func parseSelector(name string) (string, string) {
	idx := strings.LastIndex(name, ":")
	if idx <= 0 || idx >= len(name)-1 {
		return name, ""
	}
	return name[:idx], name[idx+1:]
}

func matchesPath(name, path string, recursive bool) bool {
	if path == "/" {
		if !recursive {
			trimmed := strings.TrimPrefix(name, "/")
			return !strings.Contains(trimmed, "/")
		}
		return strings.HasPrefix(name, "/")
	}

	prefix := path + "/"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if recursive {
		return true
	}

	remainder := strings.TrimPrefix(name, prefix)
	return remainder != "" && !strings.Contains(remainder, "/")
}

func randomSuffix(length int) string {
	if length <= 0 {
		return ""
	}
	b := make([]byte, (length+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func randomVersionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	hexString := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexString[0:8], hexString[8:12], hexString[12:16], hexString[16:20], hexString[20:32])
}

func encryptValue(plain string) string {
	input := []byte(plain)
	out := make([]byte, len(input))
	for i := range input {
		out[i] = input[i] ^ secureKey[i%len(secureKey)]
	}
	return base64.StdEncoding.EncodeToString(out)
}

func decryptValue(cipher string) string {
	decoded, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil {
		return ""
	}
	out := make([]byte, len(decoded))
	for i := range decoded {
		out[i] = decoded[i] ^ secureKey[i%len(secureKey)]
	}
	return string(out)
}
