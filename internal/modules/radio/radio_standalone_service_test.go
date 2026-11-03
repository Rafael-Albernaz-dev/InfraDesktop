package radio

import (
	"context"
	"testing"
)

func TestStandalonePTPTOFUOnceSemSessao(t *testing.T) {
	r := &avulsoRemote{radioLiveRemote: &radioLiveRemote{}}
	s, q, _, base := radioHarness(t, r.radioLiveRemote)
	d := base.devices[base.link.AP]
	d.Role = RolePTP
	d.Fingerprint = ""
	d.CredentialID = radioLiveUUID()
	store := &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
	s.Store = store
	s.Remote = r
	input := RadioInput{DeviceID: d.ID, Revision: d.Revision, Operation: radioLiveUUID(), Mode: RadioOnce, Family: RadioRouterOSWirelessV6}
	view, c := s.HandleRadio(context.Background(), q, input)
	if c != OK {
		t.Fatalf("PTP avulso sem sessão foi recusado: %s", c)
	}
	s.WG.Wait()
	s.Mu.Lock()
	job := s.Jobs[view.Job.ID]
	code := job.View.Code
	s.Mu.Unlock()
	if code != OK {
		t.Fatalf("consulta falhou: %s", code)
	}
	st, _ := store.ReadEquipment(context.Background(), "synthetic")
	if st.Devices[0].Fingerprint != "SHA256:synthetic" || st.Devices[0].TrustOrigin != "automatic-tofu" {
		t.Fatal("primeiro pin não persistido", st.Devices[0].Fingerprint)
	}
	if r.calls.Load() != 1 {
		t.Fatalf("esperada uma única leitura no alvo: %d", r.calls.Load())
	}
}

type avulsoFailureRemote struct {
	*radioLiveRemote
	code      Code
	addresses []string
}

func (r *avulsoFailureRemote) ReadRadio(_ context.Context, d Device, _, _ string, _ RadioFamily, _ func() Code) (*RadioReading, Code) {
	r.addresses = append(r.addresses, d.Address)
	return nil, r.code
}
func TestStandaloneRecusaSemTentativaExtra(t *testing.T) {
	for _, failure := range []Code{TrustChanged, RadioCodeRemoteAuthFailed} {
		t.Run(string(failure), func(t *testing.T) {
			baseRemote := &radioLiveRemote{}
			s, q, _, base := radioHarness(t, baseRemote)
			d := base.devices[base.link.CPE]
			d.CredentialID = radioLiveUUID()
			store := &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
			r := &avulsoFailureRemote{radioLiveRemote: baseRemote, code: failure}
			s.Store = store
			s.Remote = r
			in := RadioInput{DeviceID: d.ID, Revision: d.Revision, Operation: radioLiveUUID(), Mode: RadioOnce, Family: RadioAirOSV6}
			v, c := s.HandleRadio(context.Background(), q, in)
			if c != OK {
				t.Fatal(c)
			}
			s.WG.Wait()
			s.Mu.Lock()
			code := s.Jobs[v.Job.ID].View.Code
			s.Mu.Unlock()
			if code != failure || len(r.addresses) != 1 || r.addresses[0] != d.Address {
				t.Fatalf("code=%s destinations=%v", code, r.addresses)
			}
		})
	}
}
func TestStandaloneSemCredencialNaoInspeciona(t *testing.T) {
	r := &avulsoRemote{radioLiveRemote: &radioLiveRemote{}}
	s, q, _, base := radioHarness(t, r.radioLiveRemote)
	d := base.devices[base.link.AP]
	d.Role = RolePTP
	d.Fingerprint = ""
	d.CredentialID = ""
	s.Store = &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
	s.Remote = r
	_,
		c := s.HandleRadio(context.Background(),
		q,
		RadioInput{DeviceID: d.ID,
			Revision:  d.Revision,
			Operation: radioLiveUUID(),
			Mode:      RadioOnce,
			Family:    RadioRouterOSWirelessV6})
	if c != MissingCredential || r.calls.Load() != 0 || r.inspections.Load() != 0 {
		t.Fatalf("credencial ausente: %s, leituras %d", c, r.calls.Load())
	}
}

type avulsoFamilyRemote struct {
	*radioLiveRemote
	family RadioFamily
}

func (r *avulsoFamilyRemote) ReadRadio(ctx context.Context, d Device, u, p string, f RadioFamily, check func() Code) (*RadioReading, Code) {
	r.family = f
	return r.radioLiveRemote.ReadRadio(ctx, d, u, p, f, check)
}
func TestStandaloneNaoAceitaDriverSugeridoPeloCliente(t *testing.T) {
	baseRemote := &radioLiveRemote{}
	s, q, _, base := radioHarness(t, baseRemote)
	d := base.devices[base.link.CPE]
	d.CredentialID = radioLiveUUID()
	s.Store = &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
	r := &avulsoFamilyRemote{radioLiveRemote: baseRemote}
	s.Remote = r
	_,
		c := s.HandleRadio(context.Background(),
		q,
		RadioInput{DeviceID: d.ID,
			Revision:  d.Revision,
			Operation: radioLiveUUID(),
			Mode:      RadioOnce,
			Family:    RadioIntelbrasWOM})
	if c != OK {
		t.Fatal(c)
	}
	s.WG.Wait()
	if r.family != RadioStandalone {
		t.Fatalf("cliente escolheu diretamente o driver: %s", r.family)
	}
}

type avulsoUnknownRemote struct{ *radioLiveRemote }

func (r *avulsoUnknownRemote) ReadRadio(_ context.Context, _ Device, _, _ string, _ RadioFamily, _ func() Code) (*RadioReading, Code) {
	return &RadioReading{Family: RadioUnsupported,
			DriverReason: "Fabricante e família não identificados",
			Perspective:  "CPE"},
		RadioCodeUnsupported
}
func TestStandaloneInconclusivoNaoSobrescrevePerfil(t *testing.T) {
	baseRemote := &radioLiveRemote{}
	s, q, _, base := radioHarness(t, baseRemote)
	d := base.devices[base.link.CPE]
	d.CredentialID = radioLiveUUID()
	s.Store = &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
	s.Remote = &avulsoUnknownRemote{baseRemote}
	v,
		c := s.HandleRadio(context.Background(),
		q,
		RadioInput{DeviceID: d.ID,
			Revision:  d.Revision,
			Operation: radioLiveUUID(),
			Mode:      RadioOnce,
			Family:    RadioStandalone})
	if c != OK {
		t.Fatal(c)
	}
	s.WG.Wait()
	s.Mu.Lock()
	result := s.Jobs[v.Job.ID].Radio
	s.Mu.Unlock()
	if result.Reading == nil || result.Reading.Family != RadioUnsupported {
		t.Fatalf("identificação inconclusiva invisível: %+v", result)
	}
	if base.saves.Load() != 0 {
		t.Fatal("identificação inconclusiva substituiu leitura/perfil existente")
	}
}

type knownUnsupportedRemote struct{ *radioLiveRemote }

func (r *knownUnsupportedRemote) ReadRadio(_ context.Context, _ Device, _, _ string, _ RadioFamily, _ func() Code) (*RadioReading, Code) {
	return &RadioReading{Family: RadioUnsupported, Model: "AC synthetic", Version: "WA.v8.7.0", Perspective: "CPE"}, RadioCodeUnsupported
}
func TestStandalonePreservacaoNaoBloqueiaFallbackVinculado(t *testing.T) {
	r := &radioLiveRemote{}
	s, q, in, store := radioHarness(t, r)
	s.Remote = &knownUnsupportedRemote{r}
	d := store.devices[store.link.CPE]
	j := &runningJob{Command: q, RadioInput: in}
	reading, c := s.readRadioSide(context.Background(), j, d, 1, RadioAirOSV6)
	if c != RadioCodeUnsupported || reading == nil || store.saves.Load() != 1 {
		t.Fatalf("leitura identificada vinculada não persistiu: code=%s saves=%d", c, store.saves.Load())
	}
}
