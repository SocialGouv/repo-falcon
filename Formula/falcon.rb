class Falcon < Formula
  desc "Turn a repository into deterministic artifacts and a queryable code knowledge graph"
  homepage "https://github.com/SocialGouv/repo-falcon"
  version "${VERSION}"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/SocialGouv/repo-falcon/releases/download/v${VERSION}/falcon-darwin-arm64"
      sha256 "${SHA256_DARWIN_ARM64}"
    end
    on_intel do
      url "https://github.com/SocialGouv/repo-falcon/releases/download/v${VERSION}/falcon-darwin-amd64"
      sha256 "${SHA256_DARWIN_AMD64}"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/SocialGouv/repo-falcon/releases/download/v${VERSION}/falcon-linux-arm64"
      sha256 "${SHA256_LINUX_ARM64}"
    end
    on_intel do
      url "https://github.com/SocialGouv/repo-falcon/releases/download/v${VERSION}/falcon-linux-amd64"
      sha256 "${SHA256_LINUX_AMD64}"
    end
  end

  def install
    binary = Dir["falcon-*"].first
    mv binary, "falcon"
    bin.install "falcon"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/falcon version")
  end
end
