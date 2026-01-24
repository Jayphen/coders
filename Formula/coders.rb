class Coders < Formula
  desc "Multi-session AI coding orchestrator with tmux-based session management"
  homepage "https://github.com/Jayphen/coders"
  version "1.0.0"
  license "MIT"

  on_arm do
    url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-darwin-arm64"
    sha256 "d0c32f6074f690a9a0ab7b01fd3198e297ba07bdfee57b5f0b136dd00bf13467"
  end

  on_intel do
    url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-darwin-amd64"
    sha256 "de595088dcea1df309a57d44269e0114ade9ed67e91cc0ca96b4e4ad20f27e35"
  end

  def install
    # The downloaded binary is the executable itself
    if Hardware::CPU.arm?
      bin.install "coders-darwin-arm64" => "coders"
    elsif Hardware::CPU.intel?
      bin.install "coders-darwin-amd64" => "coders"
    end
  end

  test do
    # Test that the binary can be executed and shows version info
    assert_match "coders", shell_output("#{bin}/coders --help")
  end
end
