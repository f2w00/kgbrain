package profile

type ProfileRepository interface {
	Get(id string) (*Profile, error)
	Save(p *Profile) error
	Delete(id string) (bool, error)
}
