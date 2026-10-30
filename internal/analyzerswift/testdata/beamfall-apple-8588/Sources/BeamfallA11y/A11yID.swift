import SwiftUI

/// The single accessibility-identifier contract for the Beamfall Apple apps
/// (apple-0001 D5 / ATEST-4).
///
/// Every UI-test–reachable element gets a stable, namespaced identifier of the
/// form `<screen>.<element>`. e2e suites query *only* by these identifiers,
/// never by visible label text (which is localized) or by view index (which
/// shifts as layout evolves). Identifiers therefore ship in **Release** as
/// well as Debug — they are an automation contract, not debug scaffolding, and
/// (unlike `accessibilityLabel`) they are never read aloud by VoiceOver and are
/// not localized, so shipping them costs nothing at runtime.
///
/// This enum is the *only* sanctioned source of identifier strings. Attaching a
/// raw string literal via `.accessibilityIdentifier("…")` is banned by
/// `A11yLintTests`; use `.accessibilityID(_:)` with a case from this enum
/// instead. Adding a new screen element means adding a case here first.
///
/// The type is a shared local module (`BeamfallA11y`) under SwiftPM and is
/// compiled directly into each Xcode app target via a `project.yml` `sources:`
/// path — the same dual-compile convention `BeamfallAmbient` uses. Consumers
/// import it under the `BEAMFALL_A11Y_MODULE` flag (set only under SwiftPM).
public enum A11yID: String, CaseIterable, Sendable {
    // MARK: Mobile — enrollment / first run
    case mobileEnrollServerField = "mobile.enroll.serverField"
    case mobileEnrollPairingCodeField = "mobile.enroll.pairingCodeField"
    case mobileEnrollDeviceNameField = "mobile.enroll.deviceNameField"
    case mobileEnrollEnrollButton = "mobile.enroll.enrollButton"
    case mobileEnrollUseStoredButton = "mobile.enroll.useStoredButton"
    case mobileEnrollDemoButton = "mobile.enroll.demoButton"

    // MARK: Mobile — root tab shell
    case mobileTabHome = "mobile.tab.home"
    case mobileTabLibrary = "mobile.tab.library"
    case mobileTabSearch = "mobile.tab.search"
    case mobileTabProfile = "mobile.tab.profile"
    case mobileOfflineBanner = "mobile.offline.banner"

    // MARK: Mobile — search
    case mobileSearchField = "mobile.search.field"

    // MARK: Mobile — profile
    case mobileProfilePickerProfileButton = "mobile.profilePicker.profileButton"
    case mobileProfilePickerManagedButton = "mobile.profilePicker.managedButton"
    case mobileProfilePickerCancelButton = "mobile.profilePicker.cancelButton"
    case mobileProfileSwitchButton = "mobile.profile.switchButton"
    case mobileProfileSignOutButton = "mobile.profile.signOutButton"
    // SETTINGS-6 (ADR-0172 §4.2): the two app-lock gate toggles.
    case mobileAppLockOpenToggle = "mobile.settings.appLockOpenToggle"
    case mobileAppLockContentToggle = "mobile.settings.appLockContentToggle"

    // MARK: Mobile — reveal PIN keypad
    case mobileRevealChip = "mobile.reveal.chip"
    case mobileRevealBannerButton = "mobile.reveal.bannerButton"
    case mobilePinDigit0Button = "mobile.pin.digit0Button"
    case mobilePinDigit1Button = "mobile.pin.digit1Button"
    case mobilePinDigit2Button = "mobile.pin.digit2Button"
    case mobilePinDigit3Button = "mobile.pin.digit3Button"
    case mobilePinDigit4Button = "mobile.pin.digit4Button"
    case mobilePinDigit5Button = "mobile.pin.digit5Button"
    case mobilePinDigit6Button = "mobile.pin.digit6Button"
    case mobilePinDigit7Button = "mobile.pin.digit7Button"
    case mobilePinDigit8Button = "mobile.pin.digit8Button"
    case mobilePinDigit9Button = "mobile.pin.digit9Button"
    case mobilePinRevealButton = "mobile.pin.revealButton"
    case mobilePinCancelButton = "mobile.pin.cancelButton"

    // MARK: Mobile — detail
    case mobileBrowsePosterButton = "mobile.browse.posterButton"
    case mobileHomeHeroDetailsButton = "mobile.home.hero.detailsButton"
    case mobileDetailRoot = "mobile.detail.root"
    case mobileDetailDoneButton = "mobile.detail.doneButton"

    // MARK: Manager (macOS) — connection / enrollment
    case managerConnectServerField = "manager.connect.serverField"
    case managerConnectPairingCodeField = "manager.connect.pairingCodeField"
    case managerConnectDeviceNameField = "manager.connect.deviceNameField"
    case managerConnectEnrollButton = "manager.connect.enrollButton"
    case managerConnectConnectButton = "manager.connect.connectButton"
    case managerConnectTVCodeButton = "manager.connect.tvCodeButton"

    // MARK: Manager (macOS) — reveal toolbar
    case managerRevealPinField = "manager.reveal.pinField"
    case managerRevealRevealButton = "manager.reveal.revealButton"
    case managerRevealRelockButton = "manager.reveal.relockButton"
    case managerRevealSetPinButton = "manager.reveal.setPinButton"

    // MARK: Manager (macOS) — workbench tools
    case managerToolsHealthButton = "manager.tools.healthButton"
    case managerToolsConsoleButton = "manager.tools.consoleButton"
    case managerToolsMyDataButton = "manager.tools.myDataButton"

    // MARK: Manager (iPad/macOS) — Library Health
    case managerHealthRoot = "manager.health.root"
    case managerHealthLibraryRow = "manager.health.libraryRow"

    // MARK: Console (Manager / iPad)
    /// The Console sheet root container — e2e asserts the operator Console is
    /// present (it is dismissed via the system sheet gesture, so there is no
    /// dedicated close button to anchor).
    case consoleRoot = "console.root"
    case consoleJobsSectionButton = "console.jobs.sectionButton"
    case consoleJobsDashboard = "console.jobs.dashboard"

    // MARK: TV (Viewer)
    case tvEnrollServerField = "tv.enroll.serverField"
    case tvEnrollPairingCodeField = "tv.enroll.pairingCodeField"
    case tvEnrollDeviceNameField = "tv.enroll.deviceNameField"
    case tvEnrollEnrollButton = "tv.enroll.enrollButton"
    case tvProfilePickerList = "tv.profilePicker.list"
    case tvProfilePickerProfileButton = "tv.profilePicker.profileButton"
    case tvProfilePickerManagedButton = "tv.profilePicker.managedButton"
    case tvBrowsePosterButton = "tv.browse.posterButton"
    /// The tvOS reveal PIN entry display (a d-pad-driven digit grid feeds it;
    /// there is no text field on tvOS, so e2e reads the running PIN display).
    case tvRevealPinDisplay = "tv.reveal.pinDisplay"
    case tvRevealRevealButton = "tv.reveal.revealButton"
    case tvPlayerScrubber = "tv.player.scrubber"
    case tvPlayerForward10Button = "tv.player.forward10Button"

    // MARK: UI-test probes (DEBUG-only, AUDIT0728-NC-APPLE-INTENTS-CONTRACT-1)
    /// Hidden App Intent route-hit counters read back by the Mobile/Pad continuation UI tests.
    case uitestAppIntentRouteHitsMobile = "uitest.appIntentRouteHits.mobile"
    case uitestAppIntentRouteHitsPad = "uitest.appIntentRouteHits.pad"

    /// The screen namespace (the substring before the first `.`), e.g.
    /// `mobile`, `manager`, `console`, `tv`. Used by the resolve-smoke to group
    /// identifiers by screen and assert each is well-formed.
    public var screen: String {
        String(rawValue.prefix { $0 != "." })
    }

    /// The element segment (everything after the screen namespace).
    public var element: String {
        let parts = rawValue.split(separator: ".", maxSplits: 1)
        return parts.count == 2 ? String(parts[1]) : ""
    }
}

extension View {
    /// Attach the contract identifier for `id` to this view. This is the *only*
    /// sanctioned way to set an accessibility identifier in the Beamfall apps;
    /// raw `.accessibilityIdentifier("string")` calls are banned by the lint.
    public func accessibilityID(_ id: A11yID) -> some View {
        accessibilityIdentifier(id.rawValue)
    }
}
