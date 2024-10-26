FROM docker.io/golang:1.23.2-bookworm

# install dependencies
RUN apt-get update && \
    apt-get install -y \
    build-essential \
    git \
    libgtk-3-dev \
    libwebkit2gtk-4.0-dev \
    nsis \
    wget \
    zsh \
    && rm -rf /var/lib/apt/lists/*

# setup zsh and oh-my-zsh
RUN git clone --single-branch --depth 1 https://github.com/robbyrussell/oh-my-zsh.git ~/.oh-my-zsh
RUN cp ~/.oh-my-zsh/templates/zshrc.zsh-template ~/.zshrc
RUN chsh -s /bin/zsh

CMD [ "/bin/zsh" ]
