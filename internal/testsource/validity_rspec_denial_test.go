package testsource

import (
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// denialSpec holds request examples for GitHub issue #1366 (V1-2124): status-only denials of a
// mutating request, the same denials with a change, reload or outbound no-effect assertion, read
// denials, a custom denial matcher, a request the reader cannot name, and two non-denials.
const denialSpec = `RSpec.describe "Posts API", type: :request do
  let(:user) { create(:user) }
  it "refuses a stranger's update" do
    patch "/posts/#{post_record.id}", params: { title: "x" }
    expect(response).to have_http_status(:forbidden)
  end
  it "refuses and changes nothing" do
    expect { delete "/posts/1" }.not_to change(Post, :count)
    expect(response).to have_http_status(403)
  end
  it "refuses and keeps the title" do
    put "/posts/1", params: { title: "x" }
    expect(response).to have_http_status(:forbidden)
    expect(post_record.reload.title).to eq "old"
  end
  it "refuses and sends no mail" do
    post "/posts/1/share"
    expect(response.status).to eq 401
    expect(Mailer).not_to have_received(:deliver)
  end
  it "refuses a read" do
    get "/posts/1"
    expect(response).to have_http_status(:unauthorized)
  end
  it "refuses a head" do
    head "/posts/1"
    expect(response).to be_forbidden
  end
  it "asserts a custom denial" do
    post "/posts"
    expect(response).to render_forbidden
  end
  it "uses a helper request" do
    make_request
    expect(response).to have_http_status(:forbidden)
  end
  it "is not a denial" do
    post "/posts"
    expect(response).to have_http_status(:created)
  end
  it "negates a denial" do
    post "/posts"
    expect(response).not_to have_http_status(:forbidden)
  end
  it "quotes the status" do
    post "/posts"
    expect(response.code).to eq "403"
  end
end
`

// denialChainSpec puts the request in a before hook, in a named subject the example calls, and
// behind a let that shadows the request helper name.
const denialChainSpec = `RSpec.describe PostsController, type: :controller do
  context "hook" do
    before { post :create, params: { title: "x" } }
    it "refuses" do
      expect(response).to have_http_status(:forbidden)
    end
  end
  context "subject" do
    subject(:update_post) { put :update, params: { id: 1 } }
    it "refuses" do
      update_post
      expect(response).to have_http_status(:forbidden)
    end
  end
  context "shadowed" do
    let(:post) { create(:post) }
    it "refuses" do
      post :create
      expect(response).to have_http_status(:forbidden)
    end
  end
end
`

// TestRSpecDenialWithoutNoEffect pins TSA-V0-564 and TSA-V0-565 (V1-2124, GitHub #1366).
func TestRSpecDenialWithoutNoEffect(t *testing.T) {
	const s = "spec/requests/posts_spec.rb"
	got := rbBrief(t, s, denialSpec, bound(nil))
	wantExamples(t, got, map[int][]string{
		3:  {"RB-DENIAL-WITHOUT-NO-EFFECT advisory " + s + ":5 " + s + ":4"},
		7:  {},
		11: {},
		16: {},
		21: {},
		25: {},
		29: {"RB-DENIAL-MATCHER-CUSTOM advisory " + s + ":31 " + s + ":31"},
		33: {"RB-DENIAL-VERB-UNRESOLVED advisory " + s + ":35 " + s + ":33"},
		37: {},
		41: {},
		45: {"RB-DENIAL-WITHOUT-NO-EFFECT advisory " + s + ":47 " + s + ":46"},
	})
	examples, _ := RSpecExampleSourcesWithSupport(s, denialSpec, bound(nil))
	for _, line := range []int{3, 29, 33} {
		p := testvalidity.ApplyExampleSource(testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible}, Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, examples[line], fmt.Sprintf("%s:%d", s, line))
		if p.Hygiene.State != testvalidity.HygieneEligible {
			t.Errorf("line %d: hygiene = %+v, want ELIGIBLE: the denial reading is advisory only", line, p.Hygiene)
		}
	}
}

// TestRSpecDenialRequestOnChain pins TSA-V0-564/565 for a request in a before hook or a subject
// the example calls, and the abstention when a let shadows the request helper name.
func TestRSpecDenialRequestOnChain(t *testing.T) {
	const s = "spec/controllers/posts_controller_spec.rb"
	got := rbBrief(t, s, denialChainSpec, bound(nil))
	wantExamples(t, got, map[int][]string{
		4:  {"RB-DENIAL-WITHOUT-NO-EFFECT advisory " + s + ":5 " + s + ":3"},
		10: {"RB-DENIAL-WITHOUT-NO-EFFECT advisory " + s + ":12 " + s + ":9"},
		17: {"RB-DENIAL-VERB-UNRESOLVED advisory " + s + ":19 " + s + ":17"},
	})
}
