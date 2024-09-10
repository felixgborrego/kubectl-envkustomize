class KubectlEnvkustomize < Formula
  desc "Description of the tool"
  homepage "https://github.com/felixgborrego/kubectl-envkustomize"
  url "https://github.com/felixgborrego/kubectl-envkustomize/releases/download/v0.0.1/kubectl-envkustomize_Darwin_arm64.tar.gz"
  sha256 "818f9a8358afa1353f20a6b9cd3829fb6d33f2632ce32e4f1f4ebfefd53644f4"
  license "MIT"

  def install
    bin.install "kubectl-envkustomize"
  end
end
