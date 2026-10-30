package fiscalproduction

// ConfigurationInspection deliberately excludes file names, CNPJ, secrets and
// endpoint URLs. It is an offline installation check, not SEFAZ approval.
type ConfigurationInspection struct {
	ConfigurationValid            bool   `json:"configuration_valid"`
	CertificateAndRevocationValid bool   `json:"certificate_and_revocation_valid"`
	OfficialHomologationVerified  bool   `json:"official_homologation_verified"`
	TransmissionPerformed         bool   `json:"transmission_performed"`
	Model                         string `json:"model"`
	UF                            string `json:"uf"`
	Series                        int    `json:"series"`
	ProfileRevision               int    `json:"profile_revision"`
	ConfigurationValidUntil       string `json:"configuration_valid_until"`
	ConfigurationSHA256           string `json:"configuration_sha256"`
}

func InspectConfiguration(path string) (ConfigurationInspection, error) {
	runtime, err := Load(path)
	if err != nil {
		return ConfigurationInspection{}, ErrUnavailable
	}
	binding := runtime.Binding()
	return ConfigurationInspection{
		ConfigurationValid: true, CertificateAndRevocationValid: true,
		Model: binding.Model, UF: binding.UF, Series: binding.Series,
		ProfileRevision: binding.ProfileRevision, ConfigurationValidUntil: binding.ValidUntil,
		ConfigurationSHA256: binding.SHA256(),
	}, nil
}
