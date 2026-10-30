package main

// Transaction seams let failure/crash boundaries run without principals or
// Directory Services. The production closures retain descriptor ownership.
type readerAdmissionOps struct {
	intent  func(readerLedger) error
	create  func(*readerLedger) (int, error)
	close   func(int)
	persist func(readerLedger) error
	own     func(int, readerAudit) error
	add     func(string, string) error
	verify  func() error
	expose  func(int, readerLedger) error
}

func runReaderAdmission(l readerLedger, o readerAdmissionOps) error {
	if e := o.intent(l); e != nil {
		return e
	}
	fd, e := o.create(&l)
	if e != nil {
		return e
	}
	defer o.close(fd)
	if e = o.persist(l); e != nil {
		return e
	}
	if e = o.own(fd, l.Audit); e != nil {
		return e
	}
	for _, pair := range [][2]string{{"GroupMembership", l.Audit.ReaderName}, {"GroupMembers", l.Audit.ReaderUUID}} {
		if e = o.add(pair[0], pair[1]); e != nil {
			return e
		}
	}
	if e = o.verify(); e != nil {
		return e
	}
	l.State = "MEMBERSHIP_VERIFIED"
	if e = o.persist(l); e != nil {
		return e
	}
	if e = o.expose(fd, l); e != nil {
		return e
	}
	l.State = "ACTIVE"
	return o.persist(l)
}

type readerWithdrawalOps struct {
	restrict    func(readerLedger) error
	check       func() (map[string][]string, error)
	remove      func(string, string) error
	verifyEmpty func() error
	retire      func() error
}

func runReaderWithdrawal(l readerLedger, o readerWithdrawalOps) error {
	if e := o.restrict(l); e != nil {
		return e
	}
	group, e := o.check()
	if e != nil {
		return e
	}
	for _, pair := range [][2]string{{"GroupMembership", l.Audit.ReaderName}, {"GroupMembers", l.Audit.ReaderUUID}} {
		if len(attribute(group, pair[0])) != 0 {
			if e = o.remove(pair[0], pair[1]); e != nil {
				return e
			}
		}
	}
	if e = o.verifyEmpty(); e != nil {
		return e
	}
	return o.retire()
}

type readerRestrictionOps struct {
	binding func() error
	chmod   func() error
	owner   func() (uint32, error)
	acl     func() error
	sync    func() error
}

func runReaderRestriction(authority string, o readerRestrictionOps) error {
	if e := o.binding(); e != nil {
		return e
	}
	if e := o.chmod(); e != nil {
		return e
	}
	owner, e := o.owner()
	if e != nil {
		return e
	}
	if e = validateRestrictedReaderOwner(owner, authority); e != nil {
		return e
	}
	if e = o.acl(); e != nil {
		return e
	}
	if e = o.sync(); e != nil {
		return e
	}
	return o.binding()
}
func runRetirementArchives(evidence, principals func() error) error {
	if e := evidence(); e != nil {
		return e
	}
	return principals()
}
