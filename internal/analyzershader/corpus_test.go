package analyzershader

import (
	"crypto/sha1"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// corpusFixtures vendors the exact immutable Beamfall source bytes. Test
// execution never reads a sibling checkout or invokes Git.
//
//go:embed testdata/glsl testdata/metal
var corpusFixtures embed.FS

const (
	beamfallVisualShadersRevision = "a2c968b0d8cf4eb3e5998fa32885b626266236ee"
	beamfallAppleUIRevision       = "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6"
)

// Immutable source receipts for the complete caller-side dogfood surfaces.
// They are literals so test execution never opens a Beamfall checkout.
var pinnedCorpus = []struct{ revision, path, blob, expected string }{
	{"a2c968b0d8cf4eb3e5998fa32885b626266236ee", "shared/visual_shared.glsl", "bad225aa3abc4dfc576305ef219152278c040d11", "REJECTED|EXACT_BINDING_UNAVAILABLE|0"},
	{"a2c968b0d8cf4eb3e5998fa32885b626266236ee", "visuals/pulse.glsl", "e5e1de9b4e478d8833818eae6a76622c89d602a6", "REJECTED|EXACT_BINDING_UNAVAILABLE|0"},
	{"830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Sources/BeamfallAppleLab/Shaders/Common.metal", "7e35740db98db6b053a68b7a8269ce0f6cb758a3", "REJECTED|EXACT_BINDING_UNAVAILABLE|0"},
	{"830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Sources/BeamfallAppleLab/Shaders/Pulse.metal", "2c983e6280248d3b054db3d0f714fa8e7d436d54", "CANDIDATE||7"},
}

var glslIdentityCorpus = []string{
	"bad225aa3abc4dfc576305ef219152278c040d11 shared/visual_shared.glsl",
	"7c54b497746c438e95be0770d4d7483dca0d4869 visuals/aurora.glsl",
	"8d75aff70ae8e0afee3245026bf9d6f8094e76f5 visuals/bloom.glsl",
	"c99aa214261e2f700721c2731286fec4ba8d4d06 visuals/braid.glsl",
	"b31b02f7bd8d562f41f3c22a3cc87bebdb04ccd4 visuals/chrome.glsl",
	"fb33e24f1d7fb51192204c2b05cd765c21e59b91 visuals/circuit.glsl",
	"1333b85dd9563be082f1c238af01603cf2396d8c visuals/comets.glsl",
	"0e8ecbe3d67951454ddf9ce6649a70845e904eab visuals/constellation.glsl",
	"cf79ef78093095c0afb2c47229d1a476b9f7ffc9 visuals/crystal.glsl",
	"bed95153e68126bcd7788c619f0f0a8a03c69851 visuals/cymatics-prism.glsl",
	"19331ac3dc6e09028de08fd3ed4c2ec42f4317bf visuals/drift.glsl",
	"53a9b63c8c301fcdcd6f5b6b8ad87fb8705482c4 visuals/dunes.glsl",
	"75b073da6880394ff768e47bb88ec3dea92405b3 visuals/event-horizon.glsl",
	"cd6c0c5b1a80c9886f408917a24fbbec6462f08c visuals/filament.glsl",
	"a35b123da8624a12c793f1bb5a332c543b3ad459 visuals/glyph.glsl",
	"f49d4ca362aed8cc23f5c49ddd1ea94e97c485f2 visuals/helix.glsl",
	"ae60c5825c019dd7e1c5343018c27c482db8de51 visuals/ink.glsl",
	"bd823d7bc97577adaf67db1a31ad0a52423930a2 visuals/interference.glsl",
	"43c95831cb3427b496066a05521360a1a8eb2cad visuals/kaleidoscope.glsl",
	"35a73013742511c34c112eb1fb65ca0417596b94 visuals/laser-tunnel.glsl",
	"d1a9b7af69eb49dee0970e8073d2fcacf3835b27 visuals/lattice.glsl",
	"08377e23e933230d3c59dce180e1374b49eef3dc visuals/lens.glsl",
	"3a3f054da156bdfd762f90a2b88848b9e6815b08 visuals/liquid-optics.glsl",
	"2d48953e2be947354880de72811f404aaefafd4f visuals/loom.glsl",
	"f592b712f9af36522cf372c315cbdf23d76c257a visuals/lumen-field.glsl",
	"393e2279cf77f092416447c9ebadd7857240b4a5 visuals/memory-tunnel.glsl",
	"cc9d0d9b30bb462493a32689a6e5e259effbabf7 visuals/mirror-bloom.glsl",
	"7aa1934bac3a98818949a15ee42b8308286ff15b visuals/murmuration.glsl",
	"c786e26db1d26a73b98d211ce9124c90ee7b0c2e visuals/nebula.glsl",
	"73ce039c144eda820f04438e721bddfabaa181b5 visuals/oscillo-ring.glsl",
	"8f94cfe8243f41221024b5bca2f1cba6a994d304 visuals/particles.glsl",
	"1f4499e5415b85d19632c4898a456a2d2c32e951 visuals/petals.glsl",
	"002ef0935330b8f2d4cefe18e079434e088471dc visuals/phosphor-scope.glsl",
	"50df715c5a82015a34ee8755a3f24267a331763b visuals/photo-curtain.glsl",
	"b1596c7f7eeabaf4b4eefb63e817c63cede6c869 visuals/photo-gallery.glsl",
	"158703ec5b308829f98bb19599dfd6da27f48632 visuals/photo-glass.glsl",
	"c3c7e293f067cbbb83932816f6bddbd97fe58cd8 visuals/photo-lens.glsl",
	"6787a1c0e4719383facfc5e24e52198b0ab0856b visuals/photo-liquid.glsl",
	"16f6e763d06fdee886f77859d82fbd504f746910 visuals/photo-particles.glsl",
	"17a0c1d4f641f92703a509999776f502b46e18e3 visuals/photo-prism.glsl",
	"56945fba8dded5595edae508ac514cb7e48c8e71 visuals/photo-topo.glsl",
	"2e6ae54e2890153a96bdca22bbf72c885bc9f1d2 visuals/photo-veil.glsl",
	"8792b164b5123502574b320f6239ea270fc475be visuals/plasma.glsl",
	"6e398a0700450d3c0b61fd323f9bf85864c561a8 visuals/prism-room.glsl",
	"e5e1de9b4e478d8833818eae6a76622c89d602a6 visuals/pulse.glsl",
	"a94ed71176d2ff01ab989d414316d0a00346700f visuals/rain.glsl",
	"f589b4fe10433346f28a5ca8b95979376e37274d visuals/reactor.glsl",
	"817b32c81e4dff083241cae08016ed317af6a396 visuals/ribbons.glsl",
	"907aae580519f69d068ca6b9bbffdc9c50db83e1 visuals/ripple.glsl",
	"6883106d679a17c8d79a7974c2cbb5d1c0b34ea9 visuals/seismograph.glsl",
	"c5977005824f1d4bfcd9896941cf45971c18977a visuals/spectral-city.glsl",
	"a79b3e6fb014cb9bd792a9f83ad65878e6674da6 visuals/spectrum.glsl",
	"7a4ad74b1d48f47033883a361defda370a2e58bf visuals/starfield.glsl",
	"86b0ed80821cd155ecd60c384ce9550970132f43 visuals/strata-ridge.glsl",
	"04090e276027bd26b37a1e783c2d8aed71c935df visuals/terrain.glsl",
	"3b8df4051ef9ab54671f7bba821d6884a302b6f9 visuals/tunnel.glsl",
	"f93b47efe71e8473577f38d11aaffe27599a0e3e visuals/vault.glsl",
	"d13e29d3d276a29a3c541a979afca2f34e49ed95 visuals/vinyl.glsl",
	"8df29fba352729334e4069de9d8914fcec9cc136 visuals/volumetric-light.glsl",
	"e8ea87a8629a6c2cefede9c8a99af9efba880122 visuals/wave-cascade.glsl",
	"cc79d66fbff490335bb7b07292db0dead7de7c10 visuals/wave-tunnel.glsl",
	"25a57b387bf606842ec82252f6230dd9f7228162 visuals/waveform.glsl",
}
var metalIdentityCorpus = []string{
	"62d6db6eb42d8d9517caf8169b9bb23443be6d3b Sources/BeamfallAppleLab/Shaders/Aurora.metal",
	"fa8a6a65912567fe899ffa15d54d691bbca38b9e Sources/BeamfallAppleLab/Shaders/Bloom.metal",
	"d89ad504bd46b487e5ba76adfdd1f4222b8a2679 Sources/BeamfallAppleLab/Shaders/Braid.metal",
	"a5430497349a4422a5dc66226f56370071b225fd Sources/BeamfallAppleLab/Shaders/Chrome.metal",
	"35d9c0d9e317a83edcf5f8598154d478b3ea32a2 Sources/BeamfallAppleLab/Shaders/Circuit.metal",
	"f237489aff39b12a2d9388f84f4fe5b1596c4398 Sources/BeamfallAppleLab/Shaders/Comets.metal",
	"7e35740db98db6b053a68b7a8269ce0f6cb758a3 Sources/BeamfallAppleLab/Shaders/Common.metal",
	"540c9d69ebda7ff85b92d37efaeb81cb28c0ecc6 Sources/BeamfallAppleLab/Shaders/Constellation.metal",
	"5bd1e87c5b57eb3a7d6ad3e955fab3bf49116668 Sources/BeamfallAppleLab/Shaders/Crystal.metal",
	"f4bd2d4e8169de88d05357fc19178a6c23836e9d Sources/BeamfallAppleLab/Shaders/CuratedRuntime.metal",
	"d3fe125e0665ccfaaca96cc22be9191909e0fe87 Sources/BeamfallAppleLab/Shaders/Drift.metal",
	"aa1cb7e206abeef178285540c6fcd7bba1dbe04d Sources/BeamfallAppleLab/Shaders/Dunes.metal",
	"bb4889da0534215b5c7b4ae8da0ca4e4c9e1035f Sources/BeamfallAppleLab/Shaders/Filament.metal",
	"a999688f4ddbf292653426fba464450c8dbf7add Sources/BeamfallAppleLab/Shaders/Glyph.metal",
	"635ce6a7eef884d4e1fecdebba3d3e81bf5d6560 Sources/BeamfallAppleLab/Shaders/Helix.metal",
	"b87df68c90d3f0f9a5c806756d24466dfda85e30 Sources/BeamfallAppleLab/Shaders/Interference.metal",
	"c974f7a7c7c9eb0359fb5c0d60db99c6881308cf Sources/BeamfallAppleLab/Shaders/Kaleidoscope.metal",
	"57416e3e6b5ed9d56d9c4965c54324102db616e9 Sources/BeamfallAppleLab/Shaders/Lattice.metal",
	"1f0ea434bf512df3ca33c351f03b152d9f7ca31a Sources/BeamfallAppleLab/Shaders/MirrorBloom.metal",
	"846dd021ce72a0ae3dd1c0cf88b145cbb232b541 Sources/BeamfallAppleLab/Shaders/Nebula.metal",
	"dcd7f5090d67754eed3b67c17f4055b005d417c2 Sources/BeamfallAppleLab/Shaders/OscilloRing.metal",
	"eb51f0d8137411a97071629ff1cea07aa64728cc Sources/BeamfallAppleLab/Shaders/Particles.metal",
	"e6f01bb3e6a007f6aa4122a891be87908f17cf43 Sources/BeamfallAppleLab/Shaders/Petals.metal",
	"65129e8ed8faf08acc445b13297510447d6755ab Sources/BeamfallAppleLab/Shaders/PhosphorScope.metal",
	"0f78a8792d7e62824a68c815f3de93579e0caa67 Sources/BeamfallAppleLab/Shaders/PhotoVisuals.metal",
	"6330c78be78539808750472f12994ddc068c3734 Sources/BeamfallAppleLab/Shaders/Plasma.metal",
	"f7a7c3ab3f8c0534b2e153bc910bfa6bfc0bd817 Sources/BeamfallAppleLab/Shaders/Post.metal",
	"3210dfa42b52f77ed9b31c02026bd6b66b1e2780 Sources/BeamfallAppleLab/Shaders/PrismRoom.metal",
	"2c983e6280248d3b054db3d0f714fa8e7d436d54 Sources/BeamfallAppleLab/Shaders/Pulse.metal",
	"1717dc80a832bc3cfff928ce9b8f0fd7a3858f3d Sources/BeamfallAppleLab/Shaders/Rain.metal",
	"9d38a6a4019b528f7402011d0c5c2141ef1f923e Sources/BeamfallAppleLab/Shaders/Reactor.metal",
	"435177d0c3b9288253afba18ffd6948191a0161c Sources/BeamfallAppleLab/Shaders/Ribbons.metal",
	"474eeba1b7f1d49fbf4c71501ba1d45b6885b25f Sources/BeamfallAppleLab/Shaders/Ripple.metal",
	"f8e58a7a8c076580f7109f0640a918c3736fc6b3 Sources/BeamfallAppleLab/Shaders/Seismograph.metal",
	"c85c796e7872a215fe9fda0f70fd66b00f2ad901 Sources/BeamfallAppleLab/Shaders/Spectrum.metal",
	"921e6ec336c7e6877d3027924974397c65bcb94e Sources/BeamfallAppleLab/Shaders/Starfield.metal",
	"9d68a1157424d03afd3b2e1d22f62648760c228f Sources/BeamfallAppleLab/Shaders/StrataRidge.metal",
	"73363c7f535459b772345a0e8fd2a5d4037a09b3 Sources/BeamfallAppleLab/Shaders/Terrain.metal",
	"3bb4f97760878f4c2181d98004ac97a6b9ab8dc5 Sources/BeamfallAppleLab/Shaders/Tunnel.metal",
	"1ec525a591248ffc7d108478c1a1b27ff8f610a7 Sources/BeamfallAppleLab/Shaders/Vinyl.metal",
	"f25e3ed4394dac8a0a160e545b7fb7b8625a8636 Sources/BeamfallAppleLab/Shaders/WaveCascade.metal",
	"873af768ce204fba166aaa3bc8f99093cdc984e3 Sources/BeamfallAppleLab/Shaders/WaveTunnel.metal",
	"1d75f69198ede4a2cd7658b7ba88df5f645ac243 Sources/BeamfallAppleLab/Shaders/Waveform.metal",
	"7e35740db98db6b053a68b7a8269ce0f6cb758a3 Sources/BeamfallVisualKit/Shaders/Common.metal",
	"a90ac4e9c47ac9674392132a12de626f58e219d1 Sources/BeamfallVisualKit/Shaders/CuratedRuntime.metal",
	"42aaadb5765a6abdfd0085760ad60022b560f6b5 Sources/BeamfallVisualKit/Shaders/PaletteProbe.metal",
	"9bd74af77a39a31b75ebf35e9c87421890cf9e69 Sources/BeamfallVisualKit/Shaders/PhotoVisuals.metal",
	"5dbd8e2ed9412f21c2bd3909c920761bb0df4f87 Sources/BeamfallVisualKit/Shaders/Post.metal",
	"3210dfa42b52f77ed9b31c02026bd6b66b1e2780 Sources/BeamfallVisualKit/Shaders/PrismRoom.metal",
}

const androidVertex300Base64 = "I3ZlcnNpb24gMzAwIGVzCmxheW91dChsb2NhdGlvbj0wKSBpbiB2ZWMyIGFQb3M7Cm91dCB2ZWMyIHZVVjsKdm9pZCBtYWluKCkgewogICAgdlVWID0gdmVjMihhUG9zLnggKiAwLjUgKyAwLjUsIDAuNSAtIGFQb3MueSAqIDAuNSk7IC8vIG9yaWdpbiB0b3AtbGVmdCwgbWF0Y2hlcyBBcHBsZQogICAgZ2xfUG9zaXRpb24gPSB2ZWM0KGFQb3MsIDAuMCwgMS4wKTsKfQoK"
const pulseGLSLBase64 = "Ly8gUHVsc2Ug4oCUIExpdGUg4oCUICJCbGFjay1yb29tIFNvbGFyIENvcmUiCi8vIFBvcnRhYmxlIHBvcnQgb2YgYmVhbWZhbGwtYXBwbGUtdWkgU2hhZGVycy9QdWxzZS5tZXRhbCAoa2VlcCBpbiBzeW5jKS4KLy8gcGFyYW0wID0gR2xvdyA7IHBhcmFtMSA9IEltcGFjdCA7IHBhcmFtMiA9IENvcm9uYSBDb3VudAoKdmVjMyB2aXN1YWwodmVjMiB1diwgVmlzdWFsVW5pZm9ybXMgdSkgewogICAgdmVjMiAgcCA9IG1vdGlvbkNlbnRlcmVkKHV2LCB1KTsKICAgIGZsb2F0IHIgPSBsZW5ndGgocCk7CiAgICBmbG9hdCBhID0gYXRhbjJfKHAueSwgcC54KTsKCiAgICBmbG9hdCBnbG93QW10ICAgPSBtaXgoMC4yNSwgMS40MCwgdS5wYXJhbTApOwogICAgZmxvYXQgaW1wYWN0QW10ID0gbWl4KDAuMDYsIDAuMjYsIHUucGFyYW0xKTsKICAgIGludCAgIG51bUNvcm9uYSA9IGludChtaXgoMy4wLCA1LjAsIHUucGFyYW0yKSArIDAuNSk7CgogICAgZmxvYXQgYnJlYXRoZSA9IDAuMDA3ICogc2luKHUudGltZSAqIDEuMSkgKyAwLjAwNSAqIHNpbih1LnRpbWUgKiAyLjMgKyAwLjgpOwogICAgZmxvYXQgY29yZVIgICA9IDAuMTIgKyAwLjA2ICogdS5hbXBsaXR1ZGUgKyBicmVhdGhlICsgaW1wYWN0QW10ICogdS5iYXNzSW1wYWN0OwoKICAgIHZlYzMgY29sID0gdmVjMygwLjApOwogICAgewogICAgICAgIGZsb2F0IHJTYWZlID0gbWF4KHIsIDFlLTQpOwogICAgICAgIGZsb2F0IGlubmVyID0gc21vb3Roc3RlcChjb3JlUiwgY29yZVIgKiAwLjQwLCByU2FmZSk7CiAgICAgICAgZmxvYXQgcmltVyAgPSAwLjAxMiArIDAuMDA2ICogdS5hbXBsaXR1ZGU7CiAgICAgICAgZmxvYXQgcmltICAgPSBhYUxpbmUoclNhZmUgLSBjb3JlUiwgcmltVyk7CgogICAgICAgIHZlYzMgY29yZUNvbCA9IG1peCh2ZWMzKDEuMDAsIDAuMjAsIDAuMDIpLCB2ZWMzKDEuMDAsIDAuMDUsIDAuNjgpLAogICAgICAgICAgICAgICAgICAgICAgICAgICBzbW9vdGhzdGVwKDAuMiwgMS4wLCBpbm5lcikpOwogICAgICAgIGNvcmVDb2wgPSBtaXgoY29yZUNvbCwgdmVjMygwLjAwLCAwLjc4LCAxLjAwKSwgcmltICogMC4zNSk7CiAgICAgICAgY29yZUNvbCA9IHBlYWtXaGl0ZShjb3JlQ29sLCB1LmJhc3NJbXBhY3QgKiBpbm5lciAqIDAuMDgpOwogICAgICAgIGZsb2F0IGV4cG9zdXJlID0gMC44NSArIDEuMjUgKiB1LmxldmVsICsgMC44MCAqIHUuYmFzc0ltcGFjdDsKICAgICAgICBjb2wgKz0gY29yZUNvbCAqIChpbm5lciAqIDIuMSArIHJpbSAqIDMuNCkgKiBleHBvc3VyZTsKCiAgICAgICAgZmxvYXQgb25zZXRGbGFzaCA9IHUub25zZXQgKiA1LjAgKiBhYUxpbmUoclNhZmUgLSAoY29yZVIgKyAwLjAxOCksIDAuMDA4KTsKICAgICAgICBjb2wgKz0gdmVjMygxLjgsIDAuNDgsIDAuMDgpICogb25zZXRGbGFzaDsKICAgIH0KCiAgICB7CiAgICAgICAgZmxvYXQgc3BhY2luZyA9IDAuMDY1ICsgMC4wMjAgKiB1LmFtcGxpdHVkZTsKICAgICAgICAvLyBDb250cmFjdCBydWxlIDg6IGNvbXBpbGUtdGltZS1jb25zdGFudCBsb29wIGJvdW5kIChudW1Db3JvbmEgdG9wcyBvdXQKICAgICAgICAvLyBhdCA1KTsgYnJlYWsgb24gdGhlIGR5bmFtaWMgY291bnQgaW5zdGVhZCBvZiBsb29waW5nIG9uIGl0IGRpcmVjdGx5LgogICAgICAgIGZvciAoaW50IGkgPSAwOyBpIDwgNTsgKytpKSB7CiAgICAgICAgICAgIGlmIChpID49IG51bUNvcm9uYSkgYnJlYWs7CiAgICAgICAgICAgIGZsb2F0IGZpICAgID0gZmxvYXQoaSk7CiAgICAgICAgICAgIGZsb2F0IHJpbmdSID0gY29yZVIgKyAoZmkgKyAxLjApICogc3BhY2luZwogICAgICAgICAgICAgICAgICAgICAgICArIGltcGFjdEFtdCAqIHUuYmFzc0ltcGFjdCAqICgxLjAgLSBmaSAqIDAuMTgpOwoKICAgICAgICAgICAgZmxvYXQgbm9pc2VUICA9IGEgKiAoMC44ICsgZmkgKiAwLjE1KTsKICAgICAgICAgICAgZmxvYXQgbm9pc2VSICA9IHIgKyBmaSAqIDAuMDc7CiAgICAgICAgICAgIGZsb2F0IHR1cmIgICAgPSBmYm0odmVjMihub2lzZVQsIG5vaXNlUiArIHUudGltZSAqICgwLjA4ICsgZmkgKiAwLjAzKSkpOwogICAgICAgICAgICBmbG9hdCB0dXJiQW10ID0gMC4wMTggKyAwLjAxMCAqIHUubWlkOwogICAgICAgICAgICBmbG9hdCBkICAgICAgID0gciAtIHJpbmdSIC0gdHVyYiAqIHR1cmJBbXQ7CgogICAgICAgICAgICBmbG9hdCBoYWxmVyA9IDAuMDA2ICsgMC4wMDQgKiB1LmFtcGxpdHVkZSAtIGZpICogMC4wMDA4OwogICAgICAgICAgICBoYWxmVyAgICAgICA9IG1heChoYWxmVywgMC4wMDIpOwogICAgICAgICAgICBmbG9hdCBtYXNrICA9IGFhTGluZShkLCBoYWxmVyk7CgogICAgICAgICAgICBmbG9hdCBodWUgPSBmaSAqIDAuMjAgKyB1LmJlYXRQaGFzZSAqIDAuMzUKICAgICAgICAgICAgICAgICAgICAgICsgYSAvIDYuMjgzMTg1MyAqIDAuMTIgKyAwLjA4ICogdS5zcGVjdHJhbENlbnRyb2lkOwogICAgICAgICAgICB2ZWMzIHJpbmdDb2wgPSBwYWxWaXN1YWwoaHVlLCB1KTsKICAgICAgICAgICAgcmluZ0NvbCAgICAgID0gYWNjZW50aXplKHJpbmdDb2wsIHUuYWNjZW50LCAwLjEyKTsKCiAgICAgICAgICAgIGZsb2F0IGJyaWdodCA9IGdsb3dBbXQgKiAoMS41IC0gZmkgKiAwLjIyKSAqICgwLjkgKyB1LmFtcGxpdHVkZSAqIDAuNSkKICAgICAgICAgICAgICAgICAgICAgICAgICsgMC44ICogdS5iYXNzSW1wYWN0OwogICAgICAgICAgICBicmlnaHQgPSBtYXgoYnJpZ2h0LCAwLjApOwoKICAgICAgICAgICAgY29sICs9IHJpbmdDb2wgKiBtYXNrICogYnJpZ2h0OwogICAgICAgIH0KICAgIH0KCiAgICB7CiAgICAgICAgZmxvYXQgdHJlYmxlRSA9IHUudHJlYmxlICsgdS5mbHV4ICogMC41OwogICAgICAgIGZvciAoaW50IGkgPSAwOyBpIDwgMzI7ICsraSkgewogICAgICAgICAgICBmbG9hdCBmaSA9IGZsb2F0KGkpOwogICAgICAgICAgICB2ZWMyICBoICA9IGhhc2gyMih2ZWMyKGZpLCAzLjcpKTsKICAgICAgICAgICAgZmxvYXQgYmFuZE91dGVyID0gY29yZVIgKyBmbG9hdChudW1Db3JvbmEpICogKDAuMDY1ICsgMC4wMjAgKiB1LmFtcGxpdHVkZSkgKyAwLjA2OwogICAgICAgICAgICBmbG9hdCBzciA9IG1peChjb3JlUiArIDAuMDMsIGJhbmRPdXRlciwgaC54KTsKICAgICAgICAgICAgZmxvYXQgc2EgPSBoLnkgKiA2LjI4MzE4NTMgKyB1LnRpbWUgKiAoMC40ICsgMC42ICogaC54KSArIHUuYmVhdFBoYXNlICogMS44OwogICAgICAgICAgICB2ZWMyICBzcCA9IHZlYzIoY29zKHNhKSwgc2luKHNhKSkgKiBzcjsKICAgICAgICAgICAgZmxvYXQgZGQgPSBsZW5ndGgocCAtIHNwKTsKICAgICAgICAgICAgZmxvYXQgc3BhcmsgPSA1ZS01IC8gKGRkICogZGQgKyA1ZS01KTsKICAgICAgICAgICAgc3BhcmsgPSBtaW4oc3BhcmssIDMuMCk7CiAgICAgICAgICAgIGZsb2F0IGZsaWNrZXIgPSBoYXNoMTEoZmkgKyBmbG9vcih1LnRpbWUgKiAxOC4wKSkgKiAwLjUgKyAwLjU7CiAgICAgICAgICAgIHZlYzMgc2NvbCA9IHBhbFZpc3VhbChoLnggKyAwLjA1ICogdS5zcGVjdHJhbENlbnRyb2lkLCB1KTsKICAgICAgICAgICAgc2NvbCA9IHBlYWtXaGl0ZShzY29sLCB1Lm9uc2V0ICogdHJlYmxlRSAqIDAuMTgpOwogICAgICAgICAgICBjb2wgKz0gc2NvbCAqIHNwYXJrICogdHJlYmxlRSAqIGZsaWNrZXIgKiA0LjU7CiAgICAgICAgfQogICAgfQoKICAgIGZsb2F0IG91dGVyRmFkZSA9IGNvcmVSICsgZmxvYXQobnVtQ29yb25hKSAqIDAuMDkgKyAwLjEyOwogICAgY29sICo9IDEuMCAtIHNtb290aHN0ZXAob3V0ZXJGYWRlLCBvdXRlckZhZGUgKyAwLjE4LCByKTsKCiAgICBmbG9hdCB0cmFpbFpvb20gPSAwLjAwOCArIDAuMDE0ICogdS5iYXNzSW1wYWN0OwogICAgZmxvYXQgdHJhaWxSb3QgID0gMC4wMDQgKiB1LmJlYXQgKyAwLjAwMSAqIHUudGltZSAqIDAuMDE7CiAgICBjb2wgPSBmZWVkYmFja1RyYWlsKGNvbCwgdXYsIHUudHJhaWxEZWNheSwgdHJhaWxab29tLCB0cmFpbFJvdCk7CgogICAgcmV0dXJuIGNvbDsgLy8gTElORUFSIEhEUiDigJQgcG9zdCBjaGFpbiB0b25lbWFwcwp9Cg=="

func TestPinnedCorpusIdentityAndAndroidGeneratedGLESFacts(t *testing.T) {
	if len(glslIdentityCorpus) != 62 || len(metalIdentityCorpus) != 49 {
		t.Fatalf("inventory glsl=%d metal=%d", len(glslIdentityCorpus), len(metalIdentityCorpus))
	}
	for _, item := range append(append([]string{}, glslIdentityCorpus...), metalIdentityCorpus...) {
		if len(item) < 42 || item[40] != ' ' {
			t.Fatalf("bad inventory item %q", item)
		}
	}
	for _, item := range pinnedCorpus {
		if len(item.revision) != 40 || len(item.blob) != 40 || item.path == "" || item.expected == "" {
			t.Fatalf("bad receipt=%+v", item)
		}
	}
	source, err := base64.StdEncoding.DecodeString(androidVertex300Base64)
	if err != nil {
		t.Fatal(err)
	}
	result := Analyze(frame("beamfall.gles-3.00.vertex.android-generated", "shader.gles", string(source)))
	var decoded output
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != "CANDIDATE" || decoded.ShaderProfile != "beamfall.gles-3.00.vertex.android-generated" {
		t.Fatalf("result=%s", result)
	}
	if !hasFact(decoded.Facts, "shader.glsl.version") || !hasFact(decoded.Facts, "shader.glsl.entry") {
		t.Fatalf("facts=%+v", decoded.Facts)
	}
}

func TestPinnedCorpusReceiptsExerciseExactSourceBytes(t *testing.T) {
	for _, item := range pinnedCorpus {
		profile, family, prefix := "", "", ""
		source := []byte(nil)
		switch item.revision {
		case beamfallVisualShadersRevision:
			profile, family, prefix = "beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", "testdata/glsl/"
		case beamfallAppleUIRevision:
			profile, family, prefix = "beamfall.metal-3.1.fragment.apple-ui", "shader.metal", "testdata/metal/"
		default:
			t.Fatalf("unroutable corpus receipt=%+v", item)
		}
		if prefix != "" {
			var readErr error
			source, readErr = fs.ReadFile(corpusFixtures, prefix+item.path)
			if readErr != nil {
				t.Fatalf("read %s: %v", item.path, readErr)
			}
		}
		if got := gitBlobSHA1(source); got != item.blob {
			t.Fatalf("blob %s=%s want=%s", item.path, got, item.blob)
		}
		var result struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
			Facts  []Fact `json:"facts"`
		}
		if err := json.Unmarshal(Analyze(pinnedFixtureFrame(profile, family, item.path, source)), &result); err != nil {
			t.Fatal(err)
		}
		if got := result.Status + "|" + result.Reason + "|" + fmt.Sprint(len(result.Facts)); got != item.expected {
			t.Fatalf("receipt %s=%s want=%s", item.path, got, item.expected)
		}
	}
}

func TestPinnedCompleteBeamfallCorpora(t *testing.T) {
	type outcome struct {
		status string
		reason string
		facts  int
	}
	const rawGLSL = "REJECTED|EXACT_BINDING_UNAVAILABLE|0"
	for _, revision := range []string{beamfallVisualShadersRevision, beamfallAppleUIRevision} {
		if len(revision) != 40 {
			t.Fatalf("nonliteral corpus revision %q", revision)
		}
	}
	metalOutcomes := map[string]string{
		"Sources/BeamfallAppleLab/Shaders/Aurora.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Bloom.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Braid.metal":           "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Chrome.metal":          "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Circuit.metal":         "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Comets.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Common.metal":          "REJECTED|EXACT_BINDING_UNAVAILABLE|0",
		"Sources/BeamfallAppleLab/Shaders/Constellation.metal":   "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Crystal.metal":         "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/CuratedRuntime.metal":  "CANDIDATE||31",
		"Sources/BeamfallAppleLab/Shaders/Drift.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Dunes.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Filament.metal":        "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Glyph.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Helix.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Interference.metal":    "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Kaleidoscope.metal":    "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Lattice.metal":         "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/MirrorBloom.metal":     "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Nebula.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/OscilloRing.metal":     "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Particles.metal":       "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Petals.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/PhosphorScope.metal":   "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/PhotoVisuals.metal":    "CANDIDATE||61",
		"Sources/BeamfallAppleLab/Shaders/Plasma.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Post.metal":            "CANDIDATE||23",
		"Sources/BeamfallAppleLab/Shaders/PrismRoom.metal":       "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Pulse.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Rain.metal":            "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Reactor.metal":         "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Ribbons.metal":         "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Ripple.metal":          "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Seismograph.metal":     "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Spectrum.metal":        "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Starfield.metal":       "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/StrataRidge.metal":     "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Terrain.metal":         "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/Tunnel.metal":          "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Vinyl.metal":           "CANDIDATE||7",
		"Sources/BeamfallAppleLab/Shaders/WaveCascade.metal":     "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/WaveTunnel.metal":      "REJECTED|UNSUPPORTED_SCHEMA|0",
		"Sources/BeamfallAppleLab/Shaders/Waveform.metal":        "CANDIDATE||7",
		"Sources/BeamfallVisualKit/Shaders/Common.metal":         "REJECTED|EXACT_BINDING_UNAVAILABLE|0",
		"Sources/BeamfallVisualKit/Shaders/CuratedRuntime.metal": "CANDIDATE||31",
		"Sources/BeamfallVisualKit/Shaders/PaletteProbe.metal":   "REJECTED|EXACT_BINDING_UNAVAILABLE|0",
		"Sources/BeamfallVisualKit/Shaders/PhotoVisuals.metal":   "CANDIDATE||19",
		"Sources/BeamfallVisualKit/Shaders/Post.metal":           "CANDIDATE||23",
		"Sources/BeamfallVisualKit/Shaders/PrismRoom.metal":      "REJECTED|UNSUPPORTED_SCHEMA|0",
	}
	testCorpus := func(family, profile, prefix string, inventory []string, expected func(string) string) {
		t.Helper()
		for _, receipt := range inventory {
			blob, path, ok := strings.Cut(receipt, " ")
			if !ok || len(blob) != 40 {
				t.Fatalf("invalid receipt %q", receipt)
			}
			source, err := fs.ReadFile(corpusFixtures, prefix+path)
			if err != nil {
				t.Fatalf("missing pinned source %s: %v", path, err)
			}
			if got := gitBlobSHA1(source); got != blob {
				t.Fatalf("byte identity %s=%s want=%s", path, got, blob)
			}
			result := Analyze(frame(profile, family, string(source)))
			var decoded struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
				Facts  []Fact `json:"facts"`
			}
			if err := json.Unmarshal(result, &decoded); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			want := expected(path)
			got := decoded.Status + "|" + decoded.Reason + "|" + fmt.Sprint(len(decoded.Facts))
			if got != want {
				t.Fatalf("golden %s=%s want=%s", path, got, want)
			}
		}
	}
	testCorpus("shader.glsl", "beamfall.glsl-es-3.00.fragment.visual-shaders", "testdata/glsl/", glslIdentityCorpus, func(string) string { return rawGLSL })
	testCorpus("shader.metal", "beamfall.metal-3.1.fragment.apple-ui", "testdata/metal/", metalIdentityCorpus, func(path string) string {
		if value, ok := metalOutcomes[path]; ok {
			return value
		}
		t.Fatalf("missing literal metal golden for %s", path)
		return ""
	})
}

func gitBlobSHA1(source []byte) string {
	content := append([]byte(fmt.Sprintf("blob %d\000", len(source))), source...)
	sum := sha1.Sum(content)
	return hex.EncodeToString(sum[:])
}

func TestPinnedRawVisualBodyRejectsWithoutWrapperVersion(t *testing.T) {
	source, err := base64.StdEncoding.DecodeString(pulseGLSLBase64)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(source); hex.EncodeToString(sum[:]) != "87566733874592f9197c0c3d49183216c9fbcd2ad586226b0951e4357e951a39" {
		t.Fatalf("pulse digest=%x", sum)
	}
	got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", string(source))))
	if !strings.Contains(got, `"reason":"EXACT_BINDING_UNAVAILABLE"`) {
		t.Fatalf("raw body=%s", got)
	}
}

const literalAndroidVertexFacts = `[
{"kind":"shader.glsl.entry","input_handle":"input-1","related_handle":"-","subject":"kit/src/main/kotlin/com/beamfall/kit/visuals/AndroidGlesShaders.kt","predicate":"declares-entry","value":"vertex","instance_id":"pinned-unit","span":{"start":{"byte":63,"line":4,"column":1},"end":{"byte":76,"line":4,"column":14}},"witness_base64":"dm9pZCBtYWluKCkgew==","witness_sha256":"sha256:c8d8540c3a26168ded339cf2c20e1b79956c9ccc65f607d67ad9a1b4729c46a4","evidence_sha256":"sha256:4fbb66db56e3e0378062978ea0fb4e06e2ef4de4a35e75ddcf717512fa4f5fdd"},
{"kind":"shader.glsl.interface","input_handle":"input-1","related_handle":"-","subject":"aPos","predicate":"declares-in","value":"vec2","instance_id":"pinned-unit","span":{"start":{"byte":35,"line":2,"column":20},"end":{"byte":48,"line":2,"column":33}},"witness_base64":"aW4gdmVjMiBhUG9zOw==","witness_sha256":"sha256:9306ff27a6ab09e2fc52ef0fdf9188b5854cdf4698a7a3a99f938e5bddaabbb0","evidence_sha256":"sha256:436875e739543754f66d52c24bddceeedd81c346cc6997b2a438e0f3933529b5"},
{"kind":"shader.glsl.interface","input_handle":"input-1","related_handle":"-","subject":"vUV","predicate":"declares-out","value":"vec2","instance_id":"pinned-unit","span":{"start":{"byte":49,"line":3,"column":1},"end":{"byte":62,"line":3,"column":14}},"witness_base64":"b3V0IHZlYzIgdlVWOw==","witness_sha256":"sha256:c7c0ff24263e8854b9cc7f0a63b1c47272116808e648a2dc5cb43d197ffff1e4","evidence_sha256":"sha256:36d804440f7b0fd2e89503f12f33827dfa931aee82efd06a5c3bbc997e55dc97"},
{"kind":"shader.glsl.layout","input_handle":"input-1","related_handle":"-","subject":"kit/src/main/kotlin/com/beamfall/kit/visuals/AndroidGlesShaders.kt","predicate":"qualifies","value":"in","instance_id":"pinned-unit","span":{"start":{"byte":16,"line":2,"column":1},"end":{"byte":34,"line":2,"column":19}},"witness_base64":"bGF5b3V0KGxvY2F0aW9uPTAp","witness_sha256":"sha256:7f2c5cd9cd343ccaec8de756aaec839e7873cf6d34bc3803cfa25518d29a5601","evidence_sha256":"sha256:1a499d7ab87b64ff0fd82466819a1ebab786559a34031d081f369bcab18a84f1"},
{"kind":"shader.glsl.version","input_handle":"input-1","related_handle":"-","subject":"kit/src/main/kotlin/com/beamfall/kit/visuals/AndroidGlesShaders.kt","predicate":"declares-version","value":"300 es","instance_id":"pinned-unit","span":{"start":{"byte":0,"line":1,"column":1},"end":{"byte":15,"line":1,"column":16}},"witness_base64":"I3ZlcnNpb24gMzAwIGVz","witness_sha256":"sha256:71377bb5e8b2b6de949dba2313f12d49c4c402d89ea81ff1b011f8dd074f0758","evidence_sha256":"sha256:fa9f2754d705cbbbc37b7d27af2b8897026a48868057fb75735527c52f6862f1"}
]`

const literalApplePulseFacts = `[
{"kind":"shader.metal.attribute","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"uses-attribute","value":"buffer","instance_id":"pinned-unit","span":{"start":{"byte":681,"line":13,"column":55},"end":{"byte":694,"line":13,"column":68}},"witness_base64":"W1tidWZmZXIoMCldXQ==","witness_sha256":"sha256:7be9d5ea2afeae992bd56579764a6b4ede8ca4ad952da5bd5b56e2107e447d56","evidence_sha256":"sha256:dc001d870150f89909907b0d880709daad71f39a3af904524d118b0b705d4f0c"},
{"kind":"shader.metal.attribute","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"uses-attribute","value":"sampler","instance_id":"pinned-unit","span":{"start":{"byte":873,"line":16,"column":42},"end":{"byte":887,"line":16,"column":56}},"witness_base64":"W1tzYW1wbGVyKDApXV0=","witness_sha256":"sha256:faa2a6e2b67b86a5c1ba99e2572cbd6c62776cd5741e7ad215f25ae0ff776ff4","evidence_sha256":"sha256:d7184aca8891099f8dd4100120667a37b70163463c429d28e3cce3686b2f1e71"},
{"kind":"shader.metal.attribute","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"uses-attribute","value":"stage_in","instance_id":"pinned-unit","span":{"start":{"byte":613,"line":12,"column":36},"end":{"byte":625,"line":12,"column":48}},"witness_base64":"W1tzdGFnZV9pbl1d","witness_sha256":"sha256:d45391fe755b7c898bffd1b4110abc12cec1a5126d2525b6c132186de1b9ba54","evidence_sha256":"sha256:fd410c55290c18265240539d0608bd58075d1bf9944a2b90064672fc27ff9512"},
{"kind":"shader.metal.attribute","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"uses-attribute","value":"texture","instance_id":"pinned-unit","span":{"start":{"byte":816,"line":15,"column":53},"end":{"byte":830,"line":15,"column":67}},"witness_base64":"W1t0ZXh0dXJlKDEpXV0=","witness_sha256":"sha256:587d820aa8cda5a330f155afe9fe0b20c94150b92321c6e1a334adcf570bf350","evidence_sha256":"sha256:fcd2150c30071916a5a8f2982dd88ee1830f2e69f481c47b87fbf401965e55ef"},
{"kind":"shader.metal.attribute","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"uses-attribute","value":"texture","instance_id":"pinned-unit","span":{"start":{"byte":748,"line":14,"column":53},"end":{"byte":762,"line":14,"column":67}},"witness_base64":"W1t0ZXh0dXJlKDApXV0=","witness_sha256":"sha256:d9d4395df87f99ad95929972a5483c2e2d17a60699d6171da7623650e644ac6f","evidence_sha256":"sha256:69f9b25c25827986e3c25ced2aa01d8644870832a700c3d80a9fa834303b1afa"},
{"kind":"shader.metal.entry","input_handle":"input-1","related_handle":"-","subject":"frag_pulse","predicate":"declares-entry","value":"fragment","instance_id":"pinned-unit","span":{"start":{"byte":578,"line":12,"column":1},"end":{"byte":890,"line":16,"column":59}},"witness_base64":"ZnJhZ21lbnQgZmxvYXQ0IGZyYWdfcHVsc2UoVk91dCBpbiBbW3N0YWdlX2luXV0sCiAgICAgICAgICAgICAgICAgICAgICAgICAgIGNvbnN0YW50IFZpc3VhbFVuaWZvcm1zJiB1IFtbYnVmZmVyKDApXV0sCiAgICAgICAgICAgICAgICAgICAgICAgICAgIHRleHR1cmUyZDxmbG9hdD4gdUltYWdlICBbW3RleHR1cmUoMCldXSwKICAgICAgICAgICAgICAgICAgICAgICAgICAgdGV4dHVyZTJkPGZsb2F0PiB1UHJldiAgIFtbdGV4dHVyZSgxKV1dLAogICAgICAgICAgICAgICAgICAgICAgICAgICBzYW1wbGVyIHVTYW1wIFtbc2FtcGxlcigwKV1dKSB7","witness_sha256":"sha256:aaa8dd02da614a149d1c09e9315e162efe2d729b380bf8f4d878aba322ad39e5","evidence_sha256":"sha256:9a638cd174cfa3a866683c9a97452fa28e21d97ee2e78a0fffc6a025f317db7f"},
{"kind":"shader.metal.include","input_handle":"input-1","related_handle":"-","subject":"Sources/BeamfallAppleLab/Shaders/Pulse.metal","predicate":"includes","value":"VisualShared.h","instance_id":"pinned-unit","span":{"start":{"byte":0,"line":1,"column":1},"end":{"byte":25,"line":1,"column":26}},"witness_base64":"I2luY2x1ZGUgIlZpc3VhbFNoYXJlZC5oIg==","witness_sha256":"sha256:ed861aa9052e13d6292c48f74c704e7fd808c9125a6f75b848d0b8dba6fe7591","evidence_sha256":"sha256:0dc4716d73ef02c723226327b1352b5c003ed7e31afed0edd9bba1378fc615eb"}
]`

func TestPinnedAcceptedBeamfallFixtureFullFactTuples(t *testing.T) {
	glsl, err := base64.StdEncoding.DecodeString(androidVertex300Base64)
	if err != nil {
		t.Fatal(err)
	}
	metal, err := fs.ReadFile(corpusFixtures, pinnedPulseMetalPath)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(glsl); hex.EncodeToString(sum[:]) != "1ffd1bde37c2c9740e6e7f933a9e5f1de01a3e87453bd2ef317a0ec2c2e396b8" {
		t.Fatalf("Android generated GLSL SHA-256=%x", sum)
	}
	if sum := sha256.Sum256(metal); hex.EncodeToString(sum[:]) != "9ef8af6f8a9c7efffa0410515d108ff753a74b4470937c5dba31d6bc545aad83" {
		t.Fatalf("Pulse Metal SHA-256=%x", sum)
	}
	for _, vector := range []struct {
		name, profile, family, path string
		source                      []byte
		literal                     string
	}{
		{"android-vertex-300", "beamfall.gles-3.00.vertex.android-generated", "shader.gles", "kit/src/main/kotlin/com/beamfall/kit/visuals/AndroidGlesShaders.kt", glsl, literalAndroidVertexFacts},
		{"apple-pulse", "beamfall.metal-3.1.fragment.apple-ui", "shader.metal", "Sources/BeamfallAppleLab/Shaders/Pulse.metal", metal, literalApplePulseFacts},
	} {
		t.Run(vector.name, func(t *testing.T) {
			var expected, actual []Fact
			if err := json.Unmarshal([]byte(vector.literal), &expected); err != nil {
				t.Fatalf("literal fixture: %v", err)
			}
			if err := json.Unmarshal(Analyze(pinnedFixtureFrame(vector.profile, vector.family, vector.path, vector.source)), &struct {
				Facts *[]Fact `json:"facts"`
			}{Facts: &actual}); err != nil {
				t.Fatalf("actual fixture: %v", err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("full literal fact tuple mismatch\nactual=%+v\nexpected=%+v", actual, expected)
			}
		})
	}
}

func pinnedFixtureFrame(profile, family, path string, source []byte) []byte {
	tuple := profiles[profile]
	sum := sha256.Sum256(source)
	request := Request{Profile: Profile, Family: Family, ShaderProfile: profile, ShaderLanguage: tuple.language, ShaderVersion: tuple.version, ShaderToolchain: tuple.toolchain, RequestID: "pinned-fixture", ScopeID: "beamfall", CompilationUnitID: "pinned-unit", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{{Handle: "input-1", Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(source)}}}
	frame, _ := json.Marshal(request)
	return append(frame, '\n')
}

func TestProspectiveBoundsReturnNoPartialFacts(t *testing.T) {
	var declarations strings.Builder
	declarations.WriteString("#version 300 es\n")
	for i := 0; i <= maxFacts; i++ {
		fmt.Fprintf(&declarations, "uniform float value%d;\n", i)
	}
	declarations.WriteString("void main(){}\n")
	tooMany := declarations.String()
	got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", tooMany)))
	if !strings.Contains(got, `"reason":"OUTPUT_LIMIT"`) && !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("facts bound=%s", got)
	}
	if strings.Contains(got, `"facts":`) {
		t.Fatalf("partial facts=%s", got)
	}
	tail := "#version 300 es\nvoid main(){}\n//"
	limit := tail + strings.Repeat("x", maxInput-len(tail)-1) + "\n"
	if got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", limit))); !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("input at=%s", got)
	}
	over := append([]byte(limit), []byte("xxxxx")...)
	if got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", string(over)))); !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("input over=%s", got)
	}
}

func hasFact(facts []Fact, kind string) bool {
	for _, fact := range facts {
		if fact.Kind == kind {
			return true
		}
	}
	return false
}
