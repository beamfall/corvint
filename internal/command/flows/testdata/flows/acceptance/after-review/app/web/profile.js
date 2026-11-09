// Profile page: pressing Save stores the display name.

export function save(profile, name) {
  return { ...profile, name: name.trim() };
}
