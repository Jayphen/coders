class Coders < Formula
  desc "Multi-session AI coding orchestrator with tmux-based session management"
  homepage "https://github.com/Jayphen/coders"
  version "1.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-darwin-arm64"
      sha256 "d0c32f6074f690a9a0ab7b01fd3198e297ba07bdfee57b5f0b136dd00bf13467"
    end
    on_intel do
      url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-darwin-amd64"
      sha256 "de595088dcea1df309a57d44269e0114ade9ed67e91cc0ca96b4e4ad20f27e35"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-linux-arm64"
      sha256 "a59ec7ae37697505c40cab2b5a9514439c5edf925bbeb48ee49aac320bd3b237"
    end
    on_intel do
      url "https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-linux-amd64"
      sha256 "035b2ca0dbd4085ec2534836e4e58855056dba9f59da6d77bc2d4bae8552f477"
    end
  end

  def install
    if OS.mac?
      if Hardware::CPU.arm?
        bin.install "coders-darwin-arm64" => "coders"
      else
        bin.install "coders-darwin-amd64" => "coders"
      end
    elsif OS.linux?
      if Hardware::CPU.arm?
        bin.install "coders-linux-arm64" => "coders"
      else
        bin.install "coders-linux-amd64" => "coders"
      end
    end
  end

  test do
    # Test that the binary can be executed and shows version info
    assert_match "coders", shell_output("#{bin}/coders --help")
  end
end
